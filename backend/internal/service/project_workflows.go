package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/integrations/cloudflare"
	gh "go-notes/internal/integrations/github"
	"go-notes/internal/jobs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type ProjectWorkflows struct {
	cfg          config.Config
	projects     repository.ProjectRepository
	settings     *SettingsService
	dockerRunner *DockerRunner
}

func NewProjectWorkflows(cfg config.Config, projects repository.ProjectRepository, settings *SettingsService, dockerRunner *DockerRunner) *ProjectWorkflows {
	return &ProjectWorkflows{
		cfg:          cfg,
		projects:     projects,
		settings:     settings,
		dockerRunner: dockerRunner,
	}
}

func (w *ProjectWorkflows) Register(runner *jobs.Runner) {
	runner.Register(JobTypeCreateTemplate, w.handleCreateTemplate)
	runner.Register(JobTypeDeployExisting, w.handleDeployExisting)
	runner.Register(JobTypeForwardLocal, w.handleForwardLocal)
	runner.Register(JobTypeQuickService, w.handleQuickService)
}

func (w *ProjectWorkflows) handleCreateTemplate(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req CreateTemplateRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse template request: %w", err)
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if req.Subdomain == "" {
		req.Subdomain = req.Name
	}
	if err := ValidateProjectName(req.Name); err != nil {
		return err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return err
	}

	runtimeCfg, err := w.settings.ResolveConfig(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	templateOwner := strings.TrimSpace(runtimeCfg.GitHubTemplateOwner)
	templateRepo := strings.TrimSpace(runtimeCfg.GitHubTemplateRepo)
	templatePrivate := runtimeCfg.GitHubRepoPrivate
	if w.settings != nil {
		selection, err := w.settings.ResolveTemplateSelection(ctx, req.Template)
		if err != nil {
			return err
		}
		templateOwner = selection.Owner
		templateRepo = selection.Repo
		templatePrivate = selection.Private
	}
	if templateOwner == "" || templateRepo == "" {
		return fmt.Errorf("template source not configured")
	}

	projectDir, err := projectPath(runtimeCfg.TemplatesDir, req.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("project already exists at %s", projectDir)
	}
	if err := os.MkdirAll(runtimeCfg.TemplatesDir, 0o755); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}

	if w.settings == nil {
		return fmt.Errorf("github app settings not configured")
	}
	appSettings, configured, err := w.settings.GitHubAppSettings(ctx)
	if err != nil {
		return fmt.Errorf("load github app settings: %w", err)
	}
	if !configured {
		return fmt.Errorf("github app settings are incomplete")
	}
	creds, err := gh.ParseAppInstallationCredentials(
		appSettings.AppID,
		appSettings.InstallationID,
		appSettings.PrivateKey,
	)
	if err != nil {
		return fmt.Errorf("github app credentials: %w", err)
	}
	githubToken, err := gh.MintInstallationToken(ctx, creds)
	if err != nil {
		return err
	}

	logger.Logf("creating GitHub repo from template for %s (%s/%s)", req.Name, templateOwner, templateRepo)
	logger.Log("using GitHub App installation token for template creation")
	githubClient := gh.NewTokenClient(githubToken)
	targetOwner := runtimeCfg.GitHubRepoOwner
	if targetOwner == "" {
		targetOwner = templateOwner
	}
	logger.Logf("validating template repo access for %s/%s", templateOwner, templateRepo)
	if err := githubClient.ValidateTemplateRepo(ctx, templateOwner, templateRepo); err != nil {
		return err
	}
	repo, err := githubClient.CreateRepoFromTemplate(ctx, templateOwner, templateRepo, req.Name, targetOwner, templatePrivate)
	if err != nil {
		return err
	}
	if repo == nil {
		return fmt.Errorf("github repository response empty")
	}

	cloneURL := repo.GetCloneURL()
	if cloneURL == "" {
		return fmt.Errorf("repo clone URL missing")
	}
	authURL, err := buildAuthenticatedCloneURL(cloneURL, githubToken)
	if err != nil {
		return err
	}

	logger.Log("cloning repository into templates directory")
	if err := cloneTemplateRepo(ctx, logger, authURL, projectDir); err != nil {
		return err
	}

	proxyPort := req.ProxyPort
	dbPort := req.DBPort
	reserved := map[int]bool{}
	addDockerReservedPorts(ctx, reserved)
	if proxyPort == 0 {
		proxyPort, err = findFreePort(80, reserved)
		if err != nil {
			return err
		}
		reserved[proxyPort] = true
	}
	if dbPort == 0 {
		dbPort, err = findFreePort(5432, reserved)
		if err != nil {
			return err
		}
	}
	logger.Logf("using proxy port %d and db port %d", proxyPort, dbPort)

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := patchComposePorts(composePath, proxyPort, dbPort); err != nil {
		return err
	}
	logger.Log("patched docker-compose.yml with selected ports")

	project := models.Project{
		Name:      req.Name,
		RepoURL:   repo.GetHTMLURL(),
		Path:      projectDir,
		ProxyPort: proxyPort,
		DBPort:    dbPort,
		Status:    "provisioning",
	}
	projectRecord, err := w.upsertProject(ctx, &project)
	if err != nil {
		return err
	}

	logger.Log("starting docker compose stack")
	if err := w.runCompose(ctx, logger, projectDir); err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, runtimeCfg.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, hostname, proxyPort); err != nil {
		return err
	}

	projectRecord.Status = "running"
	if err := w.projects.Update(ctx, projectRecord); err != nil {
		return fmt.Errorf("update project status: %w", err)
	}

	return nil
}

func (w *ProjectWorkflows) handleDeployExisting(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req DeployExistingRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse deploy request: %w", err)
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateProjectName(req.Name); err != nil {
		return err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return err
	}
	if req.Port == 0 {
		req.Port = 80
	}
	if err := ValidatePort(req.Port); err != nil {
		return err
	}
	logger.Logf("using host port %d for ingress", req.Port)

	runtimeCfg, err := w.settings.ResolveConfig(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	projectDir, err := projectPath(runtimeCfg.TemplatesDir, req.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(projectDir); err != nil {
		return fmt.Errorf("project directory missing: %w", err)
	}
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("docker-compose.yml missing: %w", err)
	}

	logger.Log("starting docker compose stack")
	if err := w.runCompose(ctx, logger, projectDir); err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, runtimeCfg.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, hostname, req.Port); err != nil {
		return err
	}

	project := models.Project{
		Name:      req.Name,
		Path:      projectDir,
		ProxyPort: req.Port,
		Status:    "running",
	}
	if _, err := w.upsertProject(ctx, &project); err != nil {
		return err
	}

	return nil
}

func (w *ProjectWorkflows) handleForwardLocal(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req ForwardLocalRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse forward request: %w", err)
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateServiceName(req.Name); err != nil {
		return err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return err
	}
	if req.Port == 0 {
		req.Port = 80
	}
	if err := ValidatePort(req.Port); err != nil {
		return err
	}
	logger.Logf("forwarding localhost port %d for %s", req.Port, req.Name)

	runtimeCfg, err := w.settings.ResolveConfig(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, runtimeCfg.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, hostname, req.Port); err != nil {
		return err
	}

	return nil
}

func (w *ProjectWorkflows) handleQuickService(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req QuickServiceRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse quick service request: %w", err)
	}
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return err
	}
	if err := ValidatePort(req.Port); err != nil {
		return err
	}
	req.Image = strings.TrimSpace(req.Image)
	req.ContainerName = strings.TrimSpace(req.ContainerName)
	if req.Image == "" {
		req.Image = defaultQuickServiceImage
	}
	if req.ContainerPort == 0 {
		req.ContainerPort = defaultQuickServiceContainerPort
	}
	if err := ValidatePort(req.ContainerPort); err != nil {
		return err
	}
	if req.ContainerName != "" {
		if err := validateContainerName(req.ContainerName); err != nil {
			return err
		}
	}
	logger.Logf("using host port %d for quick service", req.Port)

	runtimeCfg, err := w.settings.ResolveConfig(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	if err := w.runContainer(ctx, logger, req); err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, runtimeCfg.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, hostname, req.Port); err != nil {
		return err
	}

	return nil
}

func (w *ProjectWorkflows) runCompose(ctx context.Context, logger jobs.Logger, projectDir string) error {
	if w.dockerRunner == nil {
		return fmt.Errorf("docker runner unavailable")
	}
	return w.dockerRunner.ComposeUp(ctx, logger, DockerComposeRequest{ProjectDir: projectDir})
}

func (w *ProjectWorkflows) runContainer(ctx context.Context, logger jobs.Logger, req QuickServiceRequest) error {
	if w.dockerRunner == nil {
		return fmt.Errorf("docker runner unavailable")
	}
	dockerReq := DockerRunRequest{
		Image:         req.Image,
		HostPort:      req.Port,
		ContainerPort: req.ContainerPort,
		ContainerName: req.ContainerName,
	}
	return w.dockerRunner.RunContainer(ctx, logger, dockerReq)
}

func (w *ProjectWorkflows) cloudflareSetup(ctx context.Context, logger jobs.Logger, cfg config.Config, cloudfl *cloudflare.Client, hostname string, port int) error {
	if cfg.Domain == "" {
		return fmt.Errorf("DOMAIN not configured")
	}
	if cloudfl == nil {
		return fmt.Errorf("cloudflare client unavailable")
	}

	logger.Logf("cloudflare settings: account_id=%s zone_id=%s tunnel=%s domain=%s", describeSetting(cfg.CloudflareAccountID), describeSetting(cfg.CloudflareZoneID), describeSetting(cfg.CloudflaredTunnel), describeSetting(cfg.Domain))
	logger.Log("updating Cloudflare DNS record")
	if err := cloudfl.EnsureDNS(ctx, hostname); err != nil {
		logger.Logf("cloudflare dns error: %v", err)
		return fmt.Errorf("cloudflare dns: %w", err)
	}
	logger.Log("updating Cloudflare tunnel ingress")
	if err := cloudfl.UpdateIngress(ctx, hostname, port); err != nil {
		logger.Logf("cloudflare ingress error: %v", err)
		return fmt.Errorf("cloudflare ingress: %w", err)
	}
	return nil
}

func (w *ProjectWorkflows) upsertProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	existing, err := w.projects.GetByName(ctx, project.Name)
	if err == nil && existing != nil {
		existing.RepoURL = project.RepoURL
		existing.Path = project.Path
		existing.ProxyPort = project.ProxyPort
		existing.DBPort = project.DBPort
		existing.Status = project.Status
		if err := w.projects.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	if err := w.projects.Create(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func projectPath(base, name string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("TEMPLATES_DIR not configured")
	}
	if err := ValidateProjectName(name); err != nil {
		return "", err
	}
	path := filepath.Join(base, name)
	return path, nil
}

func describeSetting(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "<unset>"
	}
	return trimmed
}

func findFreePort(start int, reserved map[int]bool) (int, error) {
	for port := start; port <= 65535; port++ {
		if reserved[port] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no free port available from %d", start)
}

func addDockerReservedPorts(ctx context.Context, reserved map[int]bool) {
	ports, err := listDockerPublishedPorts(ctx)
	if err != nil {
		return
	}
	for _, port := range ports {
		reserved[port] = true
	}
}

func listDockerPublishedPorts(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Ports}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parsePublishedPorts(string(output)), nil
}

func parsePublishedPorts(raw string) []int {
	seen := map[int]bool{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		for _, segment := range strings.Split(line, ",") {
			segment = strings.TrimSpace(segment)
			if !strings.Contains(segment, "->") {
				continue
			}
			parts := strings.SplitN(segment, "->", 2)
			host := strings.TrimSpace(parts[0])
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				host = host[idx+1:]
			}
			port, err := strconv.Atoi(strings.TrimSpace(host))
			if err != nil || port < 1 || port > 65535 {
				continue
			}
			seen[port] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	return ports
}

var (
	proxyPortPattern = regexp.MustCompile(`(?m)^(\s*-\s*["']?)80:80(["']?)\s*$`)
	dbPortPattern    = regexp.MustCompile(`(?m)^(\s*-\s*["']?)(?:\${DB_PORT:-5432}|5432):5432(["']?)\s*$`)
)

func patchComposePorts(path string, proxyPort, dbPort int) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read compose file: %w", err)
	}
	contents := string(raw)
	updated := contents

	if proxyPort > 0 {
		updated = proxyPortPattern.ReplaceAllString(updated, fmt.Sprintf("${1}%d:80${2}", proxyPort))
	}
	if dbPort > 0 {
		updated = dbPortPattern.ReplaceAllString(updated, fmt.Sprintf("${1}%d:5432${2}", dbPort))
	}

	if updated == contents {
		return fmt.Errorf("no port mappings updated in compose file")
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write compose file: %w", err)
	}

	return nil
}

func buildAuthenticatedCloneURL(rawURL, token string) (string, error) {
	if token == "" {
		return "", gh.ErrMissingToken
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse clone url: %w", err)
	}
	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String(), nil
}

func cloneTemplateRepo(ctx context.Context, logger jobs.Logger, authURL, projectDir string) error {
	const maxAttempts = 3
	composePath := filepath.Join(projectDir, "docker-compose.yml")

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Logf("retrying repository clone (attempt %d/%d)", attempt, maxAttempts)
		}

		if err := runQuietCommand(ctx, logger, "", []string{"GIT_TERMINAL_PROMPT=0"}, "git", "clone", authURL, projectDir); err != nil {
			lastErr = err
		} else if _, err := os.Stat(composePath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check compose file: %w", err)
		} else {
			lastErr = fmt.Errorf("docker-compose.yml missing after clone")
		}

		if err := os.RemoveAll(projectDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cleanup failed clone: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}

	return lastErr
}

func runLoggedCommand(ctx context.Context, logger jobs.Logger, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	done := make(chan error, 2)
	read := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			logger.Log(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			done <- err
			return
		}
		done <- nil
	}

	go read(stdout)
	go read(stderr)

	waitErr := cmd.Wait()
	err1 := <-done
	err2 := <-done

	if waitErr != nil {
		return fmt.Errorf("%s failed: %w", name, waitErr)
	}
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func runQuietCommand(ctx context.Context, logger jobs.Logger, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		sanitized := sanitizeGitOutput(string(output))
		if sanitized != "" {
			logger.Log(strings.TrimSpace(sanitized))
		}
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func sanitizeGitOutput(output string) string {
	re := regexp.MustCompile(`x-access-token:[^@]+@`)
	return re.ReplaceAllString(output, "x-access-token:***@")
}
