package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "bootstrap":
		runBootstrap()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gungnr bootstrap")
}

func runBootstrap() {
	var errs []error

	homeDir, cloudflaredDir, dataDir, err := defaultPaths()
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := checkExistingInstall(dataDir); err != nil {
			errs = append(errs, err)
		}

		if err := checkDirAccess("home directory", homeDir); err != nil {
			errs = append(errs, err)
		}

		if err := checkDirAccess("cloudflared directory", cloudflaredDir); err != nil {
			errs = append(errs, err)
		}

		if err := checkDirAccess("Gungnr data directory", dataDir); err != nil {
			errs = append(errs, err)
		}
	}

	if err := checkDockerAccess(); err != nil {
		errs = append(errs, err)
	}

	if err := checkDockerCompose(); err != nil {
		errs = append(errs, err)
	}

	if err := checkCloudflared(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Preflight checks failed:")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
		}
		os.Exit(1)
	}

	fmt.Println("Preflight checks passed. Starting bootstrap.")

	clientID, err := resolveGitHubClientID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub client ID required: %s\n", err.Error())
		os.Exit(1)
	}

	user, err := fetchGitHubIdentity(clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub device flow failed: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Captured GitHub identity: %s (ID %d)\n", user.Login, user.ID)

	tunnel, err := setupCloudflaredTunnel(cloudflaredDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflare tunnel setup failed: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Cloudflare tunnel ready: %s (UUID %s)\n", tunnel.Name, tunnel.ID)

	dnsSetup, err := setupCloudflareDNS(tunnel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflare DNS setup failed: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("DNS routing confirmed for %s\n", dnsSetup.Hostname)

	configPath, err := writeCloudflaredConfig(cloudflaredDir, tunnel, dnsSetup.Hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflared config generation failed: %s\n", err.Error())
		os.Exit(1)
	}

	if err := installAndStartCloudflaredService(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflared service setup failed: %s\n", err.Error())
		os.Exit(1)
	}

	if err := waitForTunnelRunning(tunnel.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflared tunnel did not report as running: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Cloudflared service is running.")

	dataPaths, err := prepareDataDir(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gungnr data directory setup failed: %s\n", err.Error())
		os.Exit(1)
	}

	githubClientSecret, err := promptNonEmpty("GitHub OAuth Client Secret: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub client secret required: %s\n", err.Error())
		os.Exit(1)
	}

	callbackDefault := fmt.Sprintf("https://%s/auth/callback", dnsSetup.Hostname)
	githubCallbackURL, err := promptWithDefault("GitHub OAuth Callback URL", callbackDefault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub callback URL required: %s\n", err.Error())
		os.Exit(1)
	}

	sessionSecret, err := generateSessionSecret(32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Session secret generation failed: %s\n", err.Error())
		os.Exit(1)
	}

	env := bootstrapEnv{
		AppEnv:              "prod",
		Port:                "8080",
		DatabaseURL:         buildDatabaseURL(defaultPostgresUser, defaultPostgresPassword, defaultPostgresDB),
		DBMaxOpenConns:      20,
		DBMaxIdleConns:      10,
		DBConnMaxLifetime:   30,
		CORSAllowedOrigins:  buildCORSOrigins(dnsSetup.Hostname),
		SessionSecret:       sessionSecret,
		SessionTTLHours:     12,
		CookieDomain:        dnsSetup.BaseDomain,
		GitHubClientID:      clientID,
		GitHubClientSecret:  githubClientSecret,
		GitHubCallbackURL:   githubCallbackURL,
		GitHubTemplateOwner: "Hrafngud",
		GitHubTemplateRepo:  "go-ground",
		GitHubRepoPrivate:   true,
		SuperUserGitHubName: user.Login,
		SuperUserGitHubID:   user.ID,
		TemplatesDir:        dataPaths.TemplatesDir,
		Domain:              dnsSetup.BaseDomain,
		CloudflareAPIToken:  dnsSetup.APIToken,
		CloudflareAccountID: dnsSetup.AccountID,
		CloudflareZoneID:    dnsSetup.ZoneID,
		CloudflareTunnelID:  tunnel.ID,
		CloudflaredConfig:   configPath,
		CloudflaredTunnel:   tunnel.Name,
		CloudflaredDir:      cloudflaredDir,
		PostgresUser:        defaultPostgresUser,
		PostgresPassword:    defaultPostgresPassword,
		PostgresDB:          defaultPostgresDB,
		ViteAPIBaseURL:      "/",
	}

	if err := env.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Generated environment is incomplete: %s\n", err.Error())
		os.Exit(1)
	}

	if err := writeEnvFile(dataPaths.EnvPath, env.Entries()); err != nil {
		fmt.Fprintf(os.Stderr, "Writing bootstrap .env failed: %s\n", err.Error())
		os.Exit(1)
	}

	composeFile, err := findComposeFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to locate docker-compose.yml: %s\n", err.Error())
		os.Exit(1)
	}

	if err := startDockerCompose(composeFile, dataPaths.EnvPath); err != nil {
		fmt.Fprintf(os.Stderr, "Docker Compose startup failed: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Waiting for API health check to pass.")
	if err := waitForAPIHealth("http://localhost/healthz", 3*time.Minute); err != nil {
		fmt.Fprintf(os.Stderr, "API health check failed: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Panel is ready: https://%s\n", dnsSetup.Hostname)
	printBootstrapSummary(dataPaths, env, dnsSetup.Hostname, configPath)
}

func defaultPaths() (string, string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("unable to resolve home directory: %w", err)
	}
	if homeDir == "" {
		return "", "", "", errors.New("home directory is empty")
	}

	cloudflaredDir := filepath.Join(homeDir, ".cloudflared")
	dataDir := filepath.Join(homeDir, "gungnr")
	return homeDir, cloudflaredDir, dataDir, nil
}

func checkExistingInstall(dataDir string) error {
	info, err := os.Stat(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to check Gungnr data directory %s: %w", dataDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("Gungnr data path %s exists but is not a directory", dataDir)
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("unable to inspect Gungnr data directory %s: %w", dataDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("existing Gungnr install detected at %s. Move or remove it before bootstrapping", dataDir)
	}

	return nil
}

func checkDirAccess(label, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(path)
			parentInfo, parentErr := os.Stat(parent)
			if parentErr != nil {
				return fmt.Errorf("%s missing at %s and unable to access parent %s: %w", label, path, parent, parentErr)
			}
			if !parentInfo.IsDir() {
				return fmt.Errorf("%s missing at %s and parent %s is not a directory", label, path, parent)
			}
			if !isWritable(parentInfo) {
				return fmt.Errorf("%s missing at %s and parent %s is not writable", label, path, parent)
			}
			return nil
		}
		return fmt.Errorf("unable to access %s at %s: %w", label, path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s at %s is not a directory", label, path)
	}

	if !isWritable(info) {
		return fmt.Errorf("%s at %s is not writable", label, path)
	}

	return nil
}

func isWritable(info os.FileInfo) bool {
	mode := info.Mode().Perm()
	return mode&0o200 != 0 || mode&0o020 != 0 || mode&0o002 != 0
}

func checkDockerAccess() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker not found in PATH. Install Docker and retry")
	}

	if _, err := runCommand("docker", "info"); err != nil {
		return fmt.Errorf("docker access failed: %w", err)
	}

	return nil
}

func checkDockerCompose() error {
	if _, err := runCommand("docker", "compose", "version"); err == nil {
		return nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := runCommand("docker-compose", "version"); runErr == nil {
			return nil
		}
	}

	return errors.New("docker compose not available. Install Docker Compose v2 (docker compose) or docker-compose")
}

func checkCloudflared() error {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return errors.New("cloudflared not found in PATH. Install cloudflared and retry")
	}

	if _, err := runCommand("cloudflared", "--version"); err != nil {
		return fmt.Errorf("cloudflared check failed: %w", err)
	}

	return nil
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), trimmed)
	}

	return trimmed, nil
}

func findComposeFile() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to resolve working directory: %w", err)
	}

	dir := startDir
	for {
		composePath := filepath.Join(dir, "docker-compose.yml")
		if info, err := os.Stat(composePath); err == nil && !info.IsDir() {
			return composePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("docker-compose.yml not found from %s upward; run bootstrap from the repo root", startDir)
}

func resolveComposeCommand() (string, []string, error) {
	if _, err := runCommand("docker", "compose", "version"); err == nil {
		return "docker", []string{"compose"}, nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := runCommand("docker-compose", "version"); runErr == nil {
			return "docker-compose", nil, nil
		}
	}

	return "", nil, errors.New("docker compose not available")
}

func startDockerCompose(composeFile, envFile string) error {
	command, baseArgs, err := resolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args := append([]string{}, baseArgs...)
	args = append(args, "--env-file", envFile, "-f", composeFile, "up", "-d", "--build")
	fmt.Println("Starting Docker Compose services.")
	return runInteractiveCommandInDir(composeDir, command, args...)
}

type tunnelInfo struct {
	ID              string
	Name            string
	CredentialsFile string
}

func setupCloudflaredTunnel(cloudflaredDir string) (*tunnelInfo, error) {
	fmt.Println("Starting Cloudflare tunnel setup.")

	if err := runInteractiveCommand("cloudflared", "tunnel", "login"); err != nil {
		return nil, fmt.Errorf("cloudflared tunnel login failed: %w", err)
	}

	credentialsPath := filepath.Join(cloudflaredDir, "cert.pem")
	if err := waitForFile(credentialsPath, 2*time.Minute); err != nil {
		return nil, fmt.Errorf("cloudflared credentials not found after login: %w", err)
	}

	tunnelName, err := promptNonEmpty("Tunnel name: ")
	if err != nil {
		return nil, err
	}

	output, err := runCommand("cloudflared", "tunnel", "create", tunnelName)
	if err != nil {
		return nil, err
	}

	tunnelID, err := parseTunnelID(output)
	if err != nil {
		return nil, err
	}

	credentialsFile := filepath.Join(cloudflaredDir, tunnelID+".json")
	if err := waitForFile(credentialsFile, 2*time.Minute); err != nil {
		return nil, fmt.Errorf("cloudflared tunnel credentials not found after create: %w", err)
	}

	return &tunnelInfo{
		ID:              tunnelID,
		Name:            tunnelName,
		CredentialsFile: credentialsFile,
	}, nil
}

type cloudflareSetup struct {
	BaseDomain string
	Hostname   string
	APIToken   string
	ZoneID     string
	AccountID  string
}

func setupCloudflareDNS(tunnel *tunnelInfo) (*cloudflareSetup, error) {
	fmt.Println("Configuring Cloudflare DNS routing.")

	baseDomain, err := promptDomain("Base domain (example.com): ")
	if err != nil {
		return nil, err
	}

	apiToken, err := promptNonEmpty("Cloudflare API token: ")
	if err != nil {
		return nil, err
	}

	zone, err := fetchCloudflareZone(apiToken, baseDomain)
	if err != nil {
		return nil, err
	}

	if err := verifyCloudflareAccountAccess(apiToken, zone.Account.ID); err != nil {
		return nil, err
	}

	hostname := fmt.Sprintf("panel.%s", baseDomain)
	fmt.Printf("Creating DNS route for %s...\n", hostname)

	if _, err := runCommand("cloudflared", "tunnel", "route", "dns", tunnel.ID, hostname); err != nil {
		return nil, err
	}

	if err := verifyDNSRecord(apiToken, zone.ID, hostname); err != nil {
		return nil, err
	}

	return &cloudflareSetup{
		BaseDomain: baseDomain,
		Hostname:   hostname,
		APIToken:   apiToken,
		ZoneID:     zone.ID,
		AccountID:  zone.Account.ID,
	}, nil
}

func writeCloudflaredConfig(cloudflaredDir string, tunnel *tunnelInfo, hostname string) (string, error) {
	if err := os.MkdirAll(cloudflaredDir, 0o755); err != nil {
		return "", fmt.Errorf("unable to create cloudflared directory: %w", err)
	}

	configPath := filepath.Join(cloudflaredDir, "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		if err := copyFile(configPath, configPath+".bak"); err != nil {
			return "", fmt.Errorf("unable to backup existing config: %w", err)
		}
	}

	credentialsFile := tunnel.CredentialsFile
	if credentialsFile == "" {
		credentialsFile = filepath.Join(cloudflaredDir, tunnel.ID+".json")
	}

	originCert := filepath.Join(cloudflaredDir, "cert.pem")
	config := fmt.Sprintf("tunnel: %s\ncredentials-file: %s\norigincert: %s\ningress:\n  - hostname: %s\n    service: http://localhost:80\n  - service: http_status:404\n", tunnel.ID, credentialsFile, originCert, hostname)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return "", fmt.Errorf("unable to write cloudflared config: %w", err)
	}

	fmt.Printf("Wrote cloudflared config to %s\n", configPath)
	return configPath, nil
}

func copyFile(src, dest string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	return output.Sync()
}

func installAndStartCloudflaredService(configPath string) error {
	fmt.Println("Installing cloudflared as a system service.")
	if err := runPrivilegedCommand("cloudflared", "--config", configPath, "service", "install"); err != nil {
		return fmt.Errorf("cloudflared service install failed: %w", err)
	}

	fmt.Println("Starting cloudflared service.")
	if err := runPrivilegedCommand("cloudflared", "--config", configPath, "service", "start"); err != nil {
		return fmt.Errorf("cloudflared service start failed: %w", err)
	}

	return nil
}

func runPrivilegedCommand(name string, args ...string) error {
	if os.Geteuid() == 0 {
		return runInteractiveCommand(name, args...)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo not found; re-run as root to execute %s %s", name, strings.Join(args, " "))
	}
	sudoArgs := append([]string{name}, args...)
	return runInteractiveCommand("sudo", sudoArgs...)
}

func waitForTunnelRunning(tunnelID string) error {
	fmt.Println("Waiting for tunnel to report active connections.")
	deadline := time.Now().Add(2 * time.Minute)
	var lastErr error
	for {
		output, err := runCommand("cloudflared", "tunnel", "info", tunnelID)
		if err == nil {
			if connections, ok := parseActiveConnections(output); ok {
				if connections > 0 {
					return nil
				}
				lastErr = fmt.Errorf("active connections reported as %d", connections)
			} else if strings.Contains(strings.ToLower(output), "status: healthy") {
				return nil
			} else {
				lastErr = fmt.Errorf("unable to confirm tunnel status from output: %s", output)
			}
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return errors.New("tunnel did not report active connections before timeout")
		}
		time.Sleep(5 * time.Second)
	}
}

func parseActiveConnections(output string) (int, bool) {
	re := regexp.MustCompile(`(?i)active connections:\s*([0-9]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

func runInteractiveCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runInteractiveCommandInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", path)
		}
		time.Sleep(2 * time.Second)
	}
}

func waitForAPIHealth(endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := http.Client{Timeout: 5 * time.Second}
	var lastErr error

	for {
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return errors.New("timed out waiting for API health")
		}

		resp, err := client.Get(endpoint)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("health check returned %s", resp.Status)
		} else {
			lastErr = err
		}

		time.Sleep(3 * time.Second)
	}
}

func promptNonEmpty(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		value, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("unable to read input: %w", err)
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value, nil
		}
		fmt.Println("Value is required.")
	}
}

func parseTunnelID(output string) (string, error) {
	re := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	match := re.FindString(output)
	if match == "" {
		return "", fmt.Errorf("unable to parse tunnel ID from cloudflared output: %s", output)
	}
	return match, nil
}

func promptDomain(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		value, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("unable to read input: %w", err)
		}
		value = strings.TrimSpace(value)
		value = strings.TrimPrefix(value, "https://")
		value = strings.TrimPrefix(value, "http://")
		value = strings.TrimSuffix(value, "/")
		value = strings.ToLower(value)
		if value == "" {
			fmt.Println("Value is required.")
			continue
		}
		if strings.Contains(value, "/") || strings.Contains(value, " ") {
			fmt.Println("Enter a base domain without paths or spaces.")
			continue
		}
		if !strings.Contains(value, ".") {
			fmt.Println("Enter a valid base domain (example.com).")
			continue
		}
		return value, nil
	}
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareZone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
}

type cloudflareZoneResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  []cloudflareZone     `json:"result"`
}

type cloudflareAccountResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type cloudflareDNSRecordResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"result"`
}

func fetchCloudflareZone(token, baseDomain string) (*cloudflareZone, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s&status=active", url.QueryEscape(baseDomain))
	body, err := cloudflareAPIRequest(token, endpoint)
	if err != nil {
		return nil, err
	}

	var payload cloudflareZoneResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unable to decode Cloudflare zone response: %w", err)
	}

	if !payload.Success {
		return nil, fmt.Errorf("Cloudflare zone lookup failed: %s", cloudflareErrors(payload.Errors))
	}

	var match *cloudflareZone
	for i := range payload.Result {
		if strings.EqualFold(payload.Result[i].Name, baseDomain) {
			match = &payload.Result[i]
			break
		}
	}

	if match == nil {
		return nil, fmt.Errorf("no active zone found for %s", baseDomain)
	}
	if match.Account.ID == "" {
		return nil, errors.New("zone response missing account id")
	}

	fmt.Printf("Validated zone %s (account %s).\n", match.Name, match.Account.Name)
	return match, nil
}

func verifyCloudflareAccountAccess(token, accountID string) error {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s", url.PathEscape(accountID))
	body, err := cloudflareAPIRequest(token, endpoint)
	if err != nil {
		return err
	}

	var payload cloudflareAccountResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("unable to decode Cloudflare account response: %w", err)
	}
	if !payload.Success {
		return fmt.Errorf("Cloudflare account lookup failed: %s", cloudflareErrors(payload.Errors))
	}
	if payload.Result.ID == "" {
		return errors.New("Cloudflare account response missing id")
	}

	fmt.Printf("Validated Cloudflare account access for %s.\n", payload.Result.Name)
	return nil
}

func verifyDNSRecord(token, zoneID, hostname string) error {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", url.PathEscape(zoneID), url.QueryEscape(hostname))
	body, err := cloudflareAPIRequest(token, endpoint)
	if err != nil {
		return err
	}

	var payload cloudflareDNSRecordResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("unable to decode Cloudflare DNS response: %w", err)
	}
	if !payload.Success {
		return fmt.Errorf("Cloudflare DNS lookup failed: %s", cloudflareErrors(payload.Errors))
	}
	if len(payload.Result) == 0 {
		return fmt.Errorf("DNS record for %s not found after routing", hostname)
	}

	return nil
}

func cloudflareAPIRequest(token, endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read Cloudflare response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare request failed: %s", strings.TrimSpace(string(body)))
	}

	return body, nil
}

func cloudflareErrors(errors []cloudflareAPIError) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		if err.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d: %s", err.Code, err.Message))
		} else if err.Message != "" {
			parts = append(parts, err.Message)
		}
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}

type githubDeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type githubAccessToken struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type githubUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

func fetchGitHubIdentity(clientID string) (*githubUser, error) {
	deviceCode, err := requestDeviceCode(clientID)
	if err != nil {
		return nil, err
	}

	printDeviceInstructions(deviceCode)

	accessToken, err := pollDeviceAccessToken(clientID, deviceCode)
	if err != nil {
		return nil, err
	}

	user, err := fetchGitHubUser(accessToken.AccessToken)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func resolveGitHubClientID() (string, error) {
	clientID := strings.TrimSpace(os.Getenv("GUNGNR_GITHUB_CLIENT_ID"))
	if clientID != "" {
		return clientID, nil
	}

	fmt.Print("GitHub OAuth Client ID for device flow: ")
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("unable to read GitHub client ID: %w", err)
	}

	clientID = strings.TrimSpace(value)
	if clientID == "" {
		return "", errors.New("GitHub client ID is required to continue")
	}

	return clientID, nil
}

func requestDeviceCode(clientID string) (*githubDeviceCode, error) {
	form := map[string]string{
		"client_id": clientID,
		"scope":     "read:user",
	}

	var payload githubDeviceCode
	if err := postFormJSON("https://github.com/login/device/code", form, &payload); err != nil {
		return nil, err
	}

	if payload.DeviceCode == "" || payload.UserCode == "" || payload.VerificationURI == "" {
		return nil, errors.New("received incomplete device authorization response from GitHub")
	}

	if payload.Interval == 0 {
		payload.Interval = 5
	}

	return &payload, nil
}

type dataPaths struct {
	Root         string
	TemplatesDir string
	StateDir     string
	EnvPath      string
}

func prepareDataDir(dataDir string) (dataPaths, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return dataPaths{}, fmt.Errorf("unable to create Gungnr data directory: %w", err)
	}

	templatesDir := filepath.Join(dataDir, "templates")
	stateDir := filepath.Join(dataDir, "state")
	for _, dir := range []string{templatesDir, stateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return dataPaths{}, fmt.Errorf("unable to create %s: %w", dir, err)
		}
	}

	envPath := filepath.Join(dataDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		return dataPaths{}, fmt.Errorf("bootstrap .env already exists at %s", envPath)
	} else if !os.IsNotExist(err) {
		return dataPaths{}, fmt.Errorf("unable to check %s: %w", envPath, err)
	}

	return dataPaths{
		Root:         dataDir,
		TemplatesDir: templatesDir,
		StateDir:     stateDir,
		EnvPath:      envPath,
	}, nil
}

const (
	defaultPostgresUser     = "notes"
	defaultPostgresPassword = "notes"
	defaultPostgresDB       = "notes"
)

type bootstrapEnv struct {
	AppEnv              string
	Port                string
	DatabaseURL         string
	DBMaxOpenConns      int
	DBMaxIdleConns      int
	DBConnMaxLifetime   int
	CORSAllowedOrigins  string
	SessionSecret       string
	SessionTTLHours     int
	CookieDomain        string
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubCallbackURL   string
	GitHubTemplateOwner string
	GitHubTemplateRepo  string
	GitHubRepoOwner     string
	GitHubRepoPrivate   bool
	SuperUserGitHubName string
	SuperUserGitHubID   int64
	TemplatesDir        string
	Domain              string
	CloudflareAPIToken  string
	CloudflareAccountID string
	CloudflareZoneID    string
	CloudflareTunnelID  string
	CloudflaredConfig   string
	CloudflaredTunnel   string
	CloudflaredDir      string
	PostgresUser        string
	PostgresPassword    string
	PostgresDB          string
	ViteAPIBaseURL      string
}

func (env bootstrapEnv) Validate() error {
	required := map[string]string{
		"SESSION_SECRET":          env.SessionSecret,
		"GITHUB_CLIENT_ID":        env.GitHubClientID,
		"GITHUB_CLIENT_SECRET":    env.GitHubClientSecret,
		"GITHUB_CALLBACK_URL":     env.GitHubCallbackURL,
		"SUPERUSER_GH_NAME":       env.SuperUserGitHubName,
		"SUPER_GH_ID":             strconv.FormatInt(env.SuperUserGitHubID, 10),
		"TEMPLATES_DIR":           env.TemplatesDir,
		"DOMAIN":                  env.Domain,
		"CLOUDFLARE_API_TOKEN":    env.CloudflareAPIToken,
		"CLOUDFLARE_ACCOUNT_ID":   env.CloudflareAccountID,
		"CLOUDFLARE_ZONE_ID":      env.CloudflareZoneID,
		"CLOUDFLARE_TUNNEL_ID":    env.CloudflareTunnelID,
		"CLOUDFLARED_CONFIG":      env.CloudflaredConfig,
		"CLOUDFLARED_TUNNEL_NAME": env.CloudflaredTunnel,
		"CLOUDFLARED_DIR":         env.CloudflaredDir,
	}

	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}

	if env.SuperUserGitHubID == 0 {
		return errors.New("SUPER_GH_ID must be non-zero")
	}
	if env.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	return nil
}

type envEntry struct {
	Key   string
	Value string
}

func (env bootstrapEnv) Entries() []envEntry {
	entries := []envEntry{
		{Key: "APP_ENV", Value: env.AppEnv},
		{Key: "PORT", Value: env.Port},
		{Key: "POSTGRES_USER", Value: env.PostgresUser},
		{Key: "POSTGRES_PASSWORD", Value: env.PostgresPassword},
		{Key: "POSTGRES_DB", Value: env.PostgresDB},
		{Key: "DATABASE_URL", Value: env.DatabaseURL},
		{Key: "DB_MAX_OPEN_CONNS", Value: strconv.Itoa(env.DBMaxOpenConns)},
		{Key: "DB_MAX_IDLE_CONNS", Value: strconv.Itoa(env.DBMaxIdleConns)},
		{Key: "DB_CONN_MAX_LIFETIME_MIN", Value: strconv.Itoa(env.DBConnMaxLifetime)},
		{Key: "CORS_ALLOWED_ORIGINS", Value: env.CORSAllowedOrigins},
		{Key: "SESSION_SECRET", Value: env.SessionSecret},
		{Key: "SESSION_TTL_HOURS", Value: strconv.Itoa(env.SessionTTLHours)},
		{Key: "COOKIE_DOMAIN", Value: env.CookieDomain},
		{Key: "SUPERUSER_GH_NAME", Value: env.SuperUserGitHubName},
		{Key: "SUPER_GH_ID", Value: strconv.FormatInt(env.SuperUserGitHubID, 10)},
		{Key: "GITHUB_CLIENT_ID", Value: env.GitHubClientID},
		{Key: "GITHUB_CLIENT_SECRET", Value: env.GitHubClientSecret},
		{Key: "GITHUB_CALLBACK_URL", Value: env.GitHubCallbackURL},
		{Key: "GITHUB_TEMPLATE_OWNER", Value: env.GitHubTemplateOwner},
		{Key: "GITHUB_TEMPLATE_REPO", Value: env.GitHubTemplateRepo},
		{Key: "GITHUB_REPO_PRIVATE", Value: strconv.FormatBool(env.GitHubRepoPrivate)},
		{Key: "TEMPLATES_DIR", Value: env.TemplatesDir},
		{Key: "DOMAIN", Value: env.Domain},
		{Key: "CLOUDFLARE_API_TOKEN", Value: env.CloudflareAPIToken},
		{Key: "CLOUDFLARE_ACCOUNT_ID", Value: env.CloudflareAccountID},
		{Key: "CLOUDFLARE_ZONE_ID", Value: env.CloudflareZoneID},
		{Key: "CLOUDFLARE_TUNNEL_ID", Value: env.CloudflareTunnelID},
		{Key: "CLOUDFLARED_CONFIG", Value: env.CloudflaredConfig},
		{Key: "CLOUDFLARED_TUNNEL_NAME", Value: env.CloudflaredTunnel},
		{Key: "CLOUDFLARED_DIR", Value: env.CloudflaredDir},
		{Key: "VITE_API_BASE_URL", Value: env.ViteAPIBaseURL},
	}

	if strings.TrimSpace(env.GitHubRepoOwner) != "" {
		entries = append(entries, envEntry{Key: "GITHUB_REPO_OWNER", Value: env.GitHubRepoOwner})
	}

	return entries
}

func writeEnvFile(path string, entries []envEntry) error {
	var builder strings.Builder
	for _, entry := range entries {
		if strings.TrimSpace(entry.Value) == "" {
			continue
		}
		builder.WriteString(entry.Key)
		builder.WriteString("=")
		builder.WriteString(formatEnvValue(entry.Value))
		builder.WriteString("\n")
	}

	return os.WriteFile(path, []byte(builder.String()), 0o600)
}

func formatEnvValue(value string) string {
	if value == "" {
		return value
	}
	if !strings.ContainsAny(value, " \t\r\n#\"'\\") {
		return value
	}
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", escaped)
}

func generateSessionSecret(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("secret length must be positive")
	}
	buffer := make([]byte, bytesLen)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("unable to generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func buildDatabaseURL(user, password, name string) string {
	return fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", url.PathEscape(user), url.PathEscape(password), url.PathEscape(name))
}

func buildCORSOrigins(hostname string) string {
	origins := []string{
		fmt.Sprintf("https://%s", hostname),
		"http://localhost:4173",
		"http://127.0.0.1:4173",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}

	seen := make(map[string]struct{}, len(origins))
	var unique []string
	for _, origin := range origins {
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		unique = append(unique, origin)
	}

	return strings.Join(unique, ",")
}

func promptWithDefault(label, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", label, defaultValue)
		} else {
			fmt.Printf("%s: ", label)
		}

		value, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("unable to read input: %w", err)
		}
		value = strings.TrimSpace(value)
		if value == "" && defaultValue != "" {
			return defaultValue, nil
		}
		if value == "" {
			fmt.Println("Value is required.")
			continue
		}
		return value, nil
	}
}

func printBootstrapSummary(paths dataPaths, env bootstrapEnv, hostname, configPath string) {
	fmt.Println("Bootstrap configuration written.")
	fmt.Printf("- Data directory: %s\n", paths.Root)
	fmt.Printf("- Templates directory: %s\n", paths.TemplatesDir)
	fmt.Printf("- State directory: %s\n", paths.StateDir)
	fmt.Printf("- .env path: %s\n", paths.EnvPath)
	fmt.Printf("- Panel hostname: https://%s\n", hostname)
	fmt.Printf("- Cloudflared config: %s\n", configPath)
	fmt.Printf("- Cloudflare tunnel: %s (%s)\n", env.CloudflaredTunnel, env.CloudflareTunnelID)
}

func printDeviceInstructions(deviceCode *githubDeviceCode) {
	fmt.Println("Authorize this machine with GitHub:")
	fmt.Printf("- Visit %s\n", deviceCode.VerificationURI)
	fmt.Printf("- Enter code: %s\n", deviceCode.UserCode)
	if deviceCode.VerificationURIComplete != "" {
		fmt.Printf("- Or open: %s\n", deviceCode.VerificationURIComplete)
	}
}

func pollDeviceAccessToken(clientID string, deviceCode *githubDeviceCode) (*githubAccessToken, error) {
	interval := time.Duration(deviceCode.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("device code expired before authorization completed")
		}

		time.Sleep(interval)

		token, err := requestAccessToken(clientID, deviceCode.DeviceCode)
		if err != nil {
			return nil, err
		}

		if token.Error == "" && token.AccessToken != "" {
			return token, nil
		}

		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied":
			return nil, errors.New("device authorization denied in GitHub")
		case "expired_token":
			return nil, errors.New("device code expired before authorization completed")
		default:
			if token.Error != "" {
				if token.ErrorDescription != "" {
					return nil, fmt.Errorf("device authorization failed: %s", token.ErrorDescription)
				}
				return nil, fmt.Errorf("device authorization failed: %s", token.Error)
			}
		}
	}
}

func requestAccessToken(clientID, deviceCode string) (*githubAccessToken, error) {
	form := map[string]string{
		"client_id":   clientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	var payload githubAccessToken
	if err := postFormJSON("https://github.com/login/oauth/access_token", form, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func fetchGitHubUser(token string) (*githubUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read GitHub user response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub user lookup failed: %s", strings.TrimSpace(string(body)))
	}

	var user githubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("unable to decode GitHub user response: %w", err)
	}

	if user.Login == "" || user.ID == 0 {
		return nil, errors.New("GitHub user response missing login or id")
	}

	return &user, nil
}

func postFormJSON(endpoint string, form map[string]string, out interface{}) error {
	values := url.Values{}
	for key, value := range form {
		values.Set(key, value)
	}
	body := values.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response from %s: %w", endpoint, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request to %s failed: %s", endpoint, strings.TrimSpace(string(payload)))
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("unable to decode response from %s: %w", endpoint, err)
	}

	return nil
}
