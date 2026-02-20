package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/infra/contract"
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
	host         *HostService
	audit        *AuditService
	dockerRunner *DockerRunner
	infraClient  infraBridgeClient
}

type cloudflareWorkflowClient interface {
	EnsureDNSForZone(ctx context.Context, hostname string, zoneID string) error
	UpdateIngress(ctx context.Context, hostname string, port int) error
}

type infraBridgeClient interface {
	RestartTunnel(ctx context.Context, requestID, configPath string) (contract.Result, error)
}

func NewProjectWorkflows(
	cfg config.Config,
	projects repository.ProjectRepository,
	settings *SettingsService,
	host *HostService,
	audit *AuditService,
	dockerRunner *DockerRunner,
	infraClient infraBridgeClient,
) *ProjectWorkflows {
	return &ProjectWorkflows{
		cfg:          cfg,
		projects:     projects,
		settings:     settings,
		host:         host,
		audit:        audit,
		dockerRunner: dockerRunner,
		infraClient:  infraClient,
	}
}

func (w *ProjectWorkflows) Register(runner *jobs.Runner) {
	runner.Register(JobTypeCreateTemplate, w.handleCreateTemplate)
	runner.Register(JobTypeDeployExisting, w.handleDeployExisting)
	runner.Register(JobTypeForwardLocal, w.handleForwardLocal)
	runner.Register(JobTypeQuickService, w.handleQuickService)
	runner.Register(JobTypeProjectArchive, w.handleProjectArchive)
}

func (w *ProjectWorkflows) handleCreateTemplate(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req CreateTemplateRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse template request: %w", err)
	}
	if w.settings == nil {
		return fmt.Errorf("settings not configured")
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
	selection, err := w.resolveDomainSelection(ctx, req.Domain)
	if err != nil {
		return err
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
	composePath, err := cloneTemplateRepo(ctx, logger, authURL, projectDir)
	if err != nil {
		return err
	}
	if err := normalizeGitOrigin(ctx, logger, projectDir, cloneURL); err != nil {
		return err
	}
	if err := repairProjectOwnership(logger, runtimeCfg.TemplatesDir, runtimeCfg.CloudflaredConfig, projectDir); err != nil {
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

	patchSummary, err := patchComposePorts(composePath, proxyPort, dbPort)
	if err == nil || patchSummary.Proxy.Matched || patchSummary.DB.Matched || patchSummary.Proxy.Reason != "" || patchSummary.DB.Reason != "" {
		logComposePatchSummary(logger, proxyPort, dbPort, patchSummary)
	}
	if err != nil {
		return err
	}

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

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, selection.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	requestID := fmt.Sprintf("job-%d", job.ID)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, requestID, hostname, selection.Domain, selection.ZoneID, proxyPort); err != nil {
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
	selection, err := w.resolveDomainSelection(ctx, req.Domain)
	if err != nil {
		return err
	}

	projectDir, err := projectPath(runtimeCfg.TemplatesDir, req.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(projectDir); err != nil {
		return fmt.Errorf("project directory missing: %w", err)
	}
	if _, err := resolveComposeFile(projectDir); err != nil {
		return fmt.Errorf("compose file missing: %w", err)
	}

	logger.Log("starting docker compose stack")
	if err := w.runCompose(ctx, logger, projectDir); err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, selection.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	requestID := fmt.Sprintf("job-%d", job.ID)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, requestID, hostname, selection.Domain, selection.ZoneID, req.Port); err != nil {
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
	selection, err := w.resolveDomainSelection(ctx, req.Domain)
	if err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, selection.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	requestID := fmt.Sprintf("job-%d", job.ID)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, requestID, hostname, selection.Domain, selection.ZoneID, req.Port); err != nil {
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
	runtimeCfg, err := w.settings.ResolveConfig(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	selection, err := w.resolveDomainSelection(ctx, req.Domain)
	if err != nil {
		return err
	}
	logger.Logf("using host port %d for quick service", req.Port)

	if err := w.runContainer(ctx, logger, req); err != nil {
		return err
	}

	hostname := fmt.Sprintf("%s.%s", req.Subdomain, selection.Domain)
	logger.Logf("configuring tunnel ingress for %s", hostname)
	cloudflareClient := cloudflare.NewClient(runtimeCfg)
	requestID := fmt.Sprintf("job-%d", job.ID)
	if err := w.cloudflareSetup(ctx, logger, runtimeCfg, cloudflareClient, requestID, hostname, selection.Domain, selection.ZoneID, req.Port); err != nil {
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

func (w *ProjectWorkflows) cloudflareSetup(ctx context.Context, logger jobs.Logger, cfg config.Config, cloudfl cloudflareWorkflowClient, requestID, hostname, domain, zoneID string, port int) error {
	if strings.TrimSpace(domain) == "" {
		return fmt.Errorf("domain not configured")
	}
	if cloudfl == nil {
		return fmt.Errorf("cloudflare client unavailable")
	}

	dnsZoneID := strings.TrimSpace(zoneID)
	if dnsZoneID == "" {
		dnsZoneID = strings.TrimSpace(cfg.CloudflareZoneID)
	}

	logger.Logf("cloudflare settings: account_id=%s zone_id=%s tunnel=%s domain=%s hostname=%s", describeSetting(cfg.CloudflareAccountID), describeSetting(dnsZoneID), describeSetting(cfg.CloudflaredTunnel), domain, hostname)
	logger.Log("updating Cloudflare DNS record")
	if err := cloudfl.EnsureDNSForZone(ctx, hostname, dnsZoneID); err != nil {
		logger.Logf("cloudflare dns error: %v", err)
		return fmt.Errorf("cloudflare dns: %w", err)
	}
	logger.Log("updating Cloudflare tunnel ingress")
	if err := cloudfl.UpdateIngress(ctx, hostname, port); err != nil {
		if errors.Is(err, cloudflare.ErrTunnelNotRemote) {
			logger.Log("tunnel is locally managed; updating local cloudflared config instead")
			if updateErr := cloudflare.UpdateLocalIngress(cfg.CloudflaredConfig, hostname, port); updateErr != nil {
				logger.Logf("cloudflared config update error: %v", updateErr)
				return fmt.Errorf("cloudflared ingress: %w", updateErr)
			}
			if w.infraClient == nil {
				logger.Log("infra bridge client unavailable; continuing after local ingress update")
				return nil
			}
			logger.Logf("submitting restart_tunnel intent via infra bridge (request_id=%s)", strings.TrimSpace(requestID))
			result, restartErr := w.infraClient.RestartTunnel(ctx, requestID, cfg.CloudflaredConfig)
			if restartErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if strings.TrimSpace(result.IntentID) != "" {
					logger.Logf("infra bridge restart_tunnel intent error: intent_id=%s", result.IntentID)
				}
				if strings.TrimSpace(result.LogPath) != "" {
					logger.Logf("infra bridge restart_tunnel log path: %s", result.LogPath)
				}
				if restartTunnelLikelyIPv6LoopbackIssue(restartErr, result) {
					logger.Log("cloudflared restart diagnostics: origin may be unreachable on IPv6 loopback (::1) while ingress uses localhost")
				}
				logger.Logf("infra bridge restart_tunnel error: %v", restartErr)
				return fmt.Errorf("cloudflared restart: %w", restartErr)
			}
			logger.Logf("infra bridge restart_tunnel intent completed: intent_id=%s status=%s", result.IntentID, result.Status)
			if strings.TrimSpace(result.LogPath) != "" {
				logger.Logf("infra bridge restart_tunnel log path: %s", result.LogPath)
			}
			if result.Error != nil {
				return fmt.Errorf("cloudflared restart failed (%s): %s", strings.TrimSpace(result.Error.Code), strings.TrimSpace(result.Error.Message))
			}
			if result.Status != contract.StatusSucceeded {
				return fmt.Errorf("cloudflared restart reported non-success status %q", result.Status)
			}
			return nil
		}
		logger.Logf("cloudflare ingress error: %v", err)
		return fmt.Errorf("cloudflare ingress: %w", err)
	}
	if updateErr := cloudflare.UpdateLocalIngress(cfg.CloudflaredConfig, hostname, port); updateErr != nil {
		logger.Logf("cloudflared config update skipped: %v", updateErr)
	}
	logger.Log("tunnel ingress updated via Cloudflare API")
	return nil
}

func (w *ProjectWorkflows) resolveDomainSelection(ctx context.Context, requested string) (DomainSelection, error) {
	if w.settings != nil {
		selection, err := w.settings.ResolveDomainSelection(ctx, requested)
		if err != nil {
			return DomainSelection{}, err
		}
		return selection, nil
	}
	base := normalizeDomain(w.cfg.Domain)
	selected, err := selectDomain(requested, base, nil)
	if err != nil {
		return DomainSelection{}, err
	}
	return DomainSelection{Domain: selected, ZoneID: strings.TrimSpace(w.cfg.CloudflareZoneID)}, nil
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

func ensureAvailableHostPort(ctx context.Context, requested int) (int, error) {
	if err := ValidatePort(requested); err != nil {
		return 0, err
	}

	reserved := map[int]bool{}
	addDockerReservedPorts(ctx, reserved)
	addHostReservedPorts(ctx, reserved)

	if !reserved[requested] {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", requested))
		if err == nil {
			_ = ln.Close()
			return requested, nil
		}
		if isPermissionError(err) {
			return requested, nil
		}
		reserved[requested] = true
	}

	port, err := findFreePortNearby(requested+1, reserved)
	if err != nil {
		return 0, err
	}
	return port, nil
}

func addHostReservedPorts(ctx context.Context, reserved map[int]bool) {
	ports, err := listHostListeningPorts(ctx)
	if err != nil {
		return
	}
	for _, port := range ports {
		reserved[port] = true
	}
}

func findFreePortNearby(start int, reserved map[int]bool) (int, error) {
	for port := start; port <= 65535; port++ {
		if reserved[port] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
		if isPermissionError(err) {
			// For privileged ports we cannot bind as a non-root user, so rely on the
			// reserved set populated from host listeners and Docker published ports.
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port available from %d", start)
}

func listHostListeningPorts(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "ss", "-ltnH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseHostListeningPorts(string(output)), nil
}

var hostPortSuffixPattern = regexp.MustCompile(`(?:\]|:)(\d+)$`)

func parseHostListeningPorts(raw string) []int {
	seen := map[int]bool{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		localAddr := strings.TrimSpace(fields[3])
		if localAddr == "" || strings.HasSuffix(localAddr, ":*") {
			continue
		}
		matches := hostPortSuffixPattern.FindStringSubmatch(localAddr)
		if matches == nil {
			continue
		}
		port, err := strconv.Atoi(matches[1])
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		seen[port] = true
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	return ports
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

type portPatchOutcome struct {
	Matched       bool
	Changed       bool
	Pattern       string
	Matches       int
	Reason        string
	ExtraMappings int
}

type composePatchSummary struct {
	Proxy portPatchOutcome
	DB    portPatchOutcome
}

func patchComposePorts(path string, proxyPort, dbPort int) (composePatchSummary, error) {
	var summary composePatchSummary
	raw, err := os.ReadFile(path)
	if err != nil {
		return summary, fmt.Errorf("read compose file: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	changed := false

	if proxyPort > 0 {
		summary.Proxy = patchComposePort(lines, 80, proxyPort)
		if summary.Proxy.Changed {
			changed = true
		}
	}
	if dbPort > 0 {
		summary.DB = patchComposePort(lines, 5432, dbPort)
		if summary.DB.Changed {
			changed = true
		}
	}

	if proxyPort > 0 && !summary.Proxy.Matched {
		return summary, fmt.Errorf("compose file missing host port mapping for container port 80")
	}

	if changed {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return summary, fmt.Errorf("write compose file: %w", err)
		}
	}

	return summary, nil
}

func patchComposePort(lines []string, containerPort, hostPort int) portPatchOutcome {
	outcome := portPatchOutcome{}
	if hostPort <= 0 {
		outcome.Reason = "host port not provided"
		return outcome
	}

	re := regexp.MustCompile(fmt.Sprintf(`^(\s*-\s*['"]?)(.+):%d(\s*/\w+)?(['"]?)(\s+#.*)?\s*$`, containerPort))
	for i, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		outcome.Matches++
		outcome.Matched = true
		if outcome.Pattern != "" {
			continue
		}

		hostPart := strings.TrimSpace(matches[2])
		newHostPart, pattern := formatHostPart(hostPart, hostPort)
		outcome.Pattern = pattern
		newLine := fmt.Sprintf("%s%s:%d%s%s%s", matches[1], newHostPart, containerPort, matches[3], matches[4], matches[5])
		if newLine != line {
			lines[i] = newLine
			outcome.Changed = true
		}
	}

	if outcome.Matches > 1 {
		outcome.ExtraMappings = outcome.Matches - 1
	}
	if !outcome.Matched {
		outcome.Reason = fmt.Sprintf("no host port mapping found for container port %d", containerPort)
	} else if !outcome.Changed {
		outcome.Reason = fmt.Sprintf("host port already set to %d for container port %d", hostPort, containerPort)
	}

	return outcome
}

func formatHostPart(hostPart string, hostPort int) (string, string) {
	hostPart = strings.TrimSpace(hostPart)
	portValue := strconv.Itoa(hostPort)

	if envVarPattern.MatchString(hostPart) {
		return portValue, "env host port"
	}

	if matches := ipv4HostPattern.FindStringSubmatch(hostPart); matches != nil {
		return fmt.Sprintf("%s:%s", matches[1], portValue), "ip-bound host port"
	}

	if strings.HasPrefix(hostPart, "localhost:") {
		return fmt.Sprintf("localhost:%s", portValue), "host-bound port"
	}

	if strings.HasPrefix(hostPart, "[") {
		if idx := strings.LastIndex(hostPart, "]:"); idx != -1 {
			return fmt.Sprintf("%s:%s", hostPart[:idx+1], portValue), "ip-bound host port"
		}
	}

	if idx := strings.LastIndex(hostPart, ":"); idx != -1 {
		return fmt.Sprintf("%s:%s", hostPart[:idx], portValue), "host-bound port"
	}

	return portValue, "explicit host port"
}

func logComposePatchSummary(logger jobs.Logger, proxyPort, dbPort int, summary composePatchSummary) {
	if proxyPort > 0 {
		logComposePatchOutcome(logger, "proxy", 80, proxyPort, summary.Proxy)
	}
	if dbPort > 0 {
		logComposePatchOutcome(logger, "db", 5432, dbPort, summary.DB)
	}
}

func logComposePatchOutcome(logger jobs.Logger, label string, containerPort, hostPort int, outcome portPatchOutcome) {
	if outcome.Matched {
		if outcome.Changed {
			logger.Logf("patched %s port using %s mapping to %d:%d", label, outcome.Pattern, hostPort, containerPort)
		} else {
			logger.Logf("%s port already set to %d:%d using %s mapping", label, hostPort, containerPort, outcome.Pattern)
		}
		if outcome.ExtraMappings > 0 {
			logger.Logf("found %d additional %s port mappings for container port %d (left unchanged)", outcome.ExtraMappings, label, containerPort)
		}
		return
	}
	if outcome.Reason != "" {
		logger.Logf("no %s port update applied: %s", label, outcome.Reason)
		return
	}
	logger.Logf("no %s port update applied for container port %d", label, containerPort)
}

var (
	envVarPattern   = regexp.MustCompile(`^\$\{[^}]+\}$`)
	ipv4HostPattern = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3}):`)
)

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

func cloneTemplateRepo(ctx context.Context, logger jobs.Logger, authURL, projectDir string) (string, error) {
	const maxAttempts = 10

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Logf("retrying repository clone (attempt %d/%d)", attempt, maxAttempts)
		}

		if err := runQuietCommand(ctx, logger, "", []string{"GIT_TERMINAL_PROMPT=0"}, "git", "clone", authURL, projectDir); err != nil {
			lastErr = err
		} else {
			composePath, resolveErr := resolveComposeFile(projectDir)
			if resolveErr == nil {
				return composePath, nil
			}
			if !errors.Is(resolveErr, os.ErrNotExist) {
				return "", resolveErr
			}
			logger.Logf(
				"clone attempt %d/%d completed but compose file is not present yet (entries: %s)",
				attempt,
				maxAttempts,
				summarizeTopLevelEntries(projectDir, 8),
			)
			lastErr = fmt.Errorf("compose file missing after clone: %w", resolveErr)
		}

		if attempt >= maxAttempts {
			break
		}

		if err := os.RemoveAll(projectDir); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("cleanup failed clone: %w", err)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt*2) * time.Second):
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("%w (final entries: %s)", lastErr, summarizeTopLevelEntries(projectDir, 12))
	}
	return "", fmt.Errorf("repository clone failed without a terminal error")
}

var composeFileCandidates = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yml",
	"docker-compose.yaml",
}

func resolveComposeFile(projectDir string) (string, error) {
	for _, name := range composeFileCandidates {
		path := filepath.Join(projectDir, name)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("check compose file %s: %w", path, err)
		}
		if info.IsDir() {
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf(
		"expected one of %s in %s: %w",
		strings.Join(composeFileCandidates, ", "),
		projectDir,
		os.ErrNotExist,
	)
}

func summarizeTopLevelEntries(projectDir string, max int) string {
	if max <= 0 {
		max = 5
	}
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return fmt.Sprintf("unreadable (%v)", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" || name == ".git" {
			continue
		}
		names = append(names, name)
		if len(names) >= max {
			break
		}
	}
	if len(names) == 0 {
		return "<none>"
	}
	if len(entries)-1 > len(names) {
		return fmt.Sprintf("%s (+more)", strings.Join(names, ", "))
	}
	return strings.Join(names, ", ")
}

func normalizeGitOrigin(ctx context.Context, logger jobs.Logger, projectDir, cloneURL string) error {
	cloneURL = strings.TrimSpace(cloneURL)
	if cloneURL == "" {
		return fmt.Errorf("repository clone URL is empty")
	}
	logger.Log("setting git origin to canonical repository URL")
	if err := runQuietCommand(ctx, logger, projectDir, nil, "git", "remote", "set-url", "origin", cloneURL); err != nil {
		return fmt.Errorf("set git remote origin: %w", err)
	}
	return nil
}

func repairProjectOwnership(logger jobs.Logger, templatesDir, cloudflaredConfig, projectDir string) error {
	if os.Geteuid() != 0 {
		return nil
	}

	uid, gid, err := detectPreferredOwnership(templatesDir, cloudflaredConfig)
	if err != nil {
		return err
	}
	if uid == 0 && gid == 0 {
		return nil
	}

	logger.Logf("applying ownership %d:%d to %s", uid, gid, projectDir)
	if err := chownRecursive(projectDir, uid, gid); err != nil {
		return err
	}
	return nil
}

func detectPreferredOwnership(templatesDir, cloudflaredConfig string) (int, int, error) {
	candidates := ownershipCandidates(templatesDir, cloudflaredConfig)
	if len(candidates) == 0 {
		return 0, 0, fmt.Errorf("templates directory not configured")
	}

	for _, candidate := range candidates {
		uid, gid, found, err := detectOwnershipFromDirectoryEntries(candidate)
		if err != nil {
			return 0, 0, err
		}
		if found {
			return uid, gid, nil
		}
	}

	for _, candidate := range candidates {
		uid, gid, found, err := detectOwnershipFromPathParents(candidate)
		if err != nil {
			return 0, 0, err
		}
		if found {
			return uid, gid, nil
		}
	}

	return 0, 0, nil
}

func ownershipCandidates(templatesDir, cloudflaredConfig string) []string {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 2)
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		clean := filepath.Clean(trimmed)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		candidates = append(candidates, clean)
	}

	add(templatesDir)
	if cfgPath := strings.TrimSpace(expandUserPath(cloudflaredConfig)); cfgPath != "" {
		add(filepath.Dir(filepath.Clean(cfgPath)))
	}

	return candidates
}

func restartTunnelLikelyIPv6LoopbackIssue(err error, result contract.Result) bool {
	if strings.Contains(strings.ToLower(err.Error()), "[::1]") {
		return true
	}
	if result.Error != nil && strings.Contains(strings.ToLower(strings.TrimSpace(result.Error.Message)), "[::1]") {
		return true
	}
	for _, line := range result.LogTail {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(normalized, "[::1]") && strings.Contains(normalized, "connect: connection refused") {
			return true
		}
	}
	return false
}

func detectOwnershipFromDirectoryEntries(path string) (int, int, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("stat ownership anchor %s: %w", path, err)
	}
	if !info.IsDir() {
		return 0, 0, false, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, false, fmt.Errorf("read ownership entries for %s: %w", path, err)
	}
	for _, entry := range entries {
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			if os.IsNotExist(infoErr) {
				continue
			}
			return 0, 0, false, fmt.Errorf("read ownership entry %s: %w", filepath.Join(path, entry.Name()), infoErr)
		}
		stat, ok := entryInfo.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		uid := int(stat.Uid)
		gid := int(stat.Gid)
		if uid != 0 || gid != 0 {
			return uid, gid, true, nil
		}
	}

	return 0, 0, false, nil
}

func detectOwnershipFromPathParents(path string) (int, int, bool, error) {
	anchor := filepath.Clean(path)
	for {
		info, err := os.Stat(anchor)
		if err == nil {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, 0, false, fmt.Errorf("read ownership for %s", anchor)
			}
			uid := int(stat.Uid)
			gid := int(stat.Gid)
			if uid != 0 || gid != 0 {
				return uid, gid, true, nil
			}
		} else if !os.IsNotExist(err) {
			return 0, 0, false, fmt.Errorf("stat ownership anchor %s: %w", anchor, err)
		}

		parent := filepath.Dir(anchor)
		if parent == anchor {
			break
		}
		anchor = parent
	}

	return 0, 0, false, nil
}

func chownRecursive(path string, uid, gid int) error {
	return filepath.WalkDir(path, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := os.Lchown(current, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", current, err)
		}
		return nil
	})
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
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			done <- err
			return
		}
		done <- nil
	}

	go read(stdout)
	go read(stderr)

	err1 := <-done
	err2 := <-done
	waitErr := cmd.Wait()

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
