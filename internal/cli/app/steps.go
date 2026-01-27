package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gungnr-cli/internal/cli/integrations/cloudflare"
	"gungnr-cli/internal/cli/integrations/cloudflared"
	"gungnr-cli/internal/cli/integrations/docker"
	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/github"
	"gungnr-cli/internal/cli/integrations/health"
	clistrings "gungnr-cli/internal/cli/strings"
	"gungnr-cli/internal/cli/validate"
)

type stepFunc struct {
	id    string
	title string
	run   func(ctx context.Context, state *State, ui UI) error
}

func (s stepFunc) ID() string    { return s.id }
func (s stepFunc) Title() string { return s.title }
func (s stepFunc) Run(ctx context.Context, state *State, ui UI) error {
	return s.run(ctx, state, ui)
}

func BootstrapSteps() []Step {
	return []Step{
		stepFunc{id: "preflight", title: "Preflight checks", run: runPreflight},
		stepFunc{id: "github_identity", title: "GitHub identity", run: runGitHubIdentity},
		stepFunc{id: "cloudflared_tunnel", title: "Cloudflared tunnel", run: runCloudflaredTunnel},
		stepFunc{id: "cloudflare_dns", title: "Cloudflare DNS", run: runCloudflareDNS},
		stepFunc{id: "cloudflared_run", title: "Cloudflared config + run", run: runCloudflaredRun},
		stepFunc{id: "env_setup", title: "Data + env setup", run: runEnvSetup},
		stepFunc{id: "compose_start", title: "Start services", run: runComposeStart},
	}
}

func runPreflight(ctx context.Context, state *State, ui UI) error {
	ui.StepProgress("preflight", "Resolving default paths")
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return err
	}

	ui.StepProgress("preflight", "Checking existing install")
	if err := filesystem.CheckExistingInstall(paths.DataDir); err != nil {
		return err
	}

	ui.StepProgress("preflight", "Checking filesystem access")
	if err := filesystem.CheckDirAccess("home directory", paths.HomeDir); err != nil {
		return err
	}
	if err := filesystem.CheckDirAccess("cloudflared directory", paths.CloudflaredDir); err != nil {
		return err
	}
	if err := filesystem.CheckDirAccess("Gungnr data directory", paths.DataDir); err != nil {
		return err
	}

	ui.StepProgress("preflight", "Checking Docker access")
	if err := docker.CheckDockerAccess(); err != nil {
		return err
	}

	ui.StepProgress("preflight", "Checking Docker Compose")
	if err := docker.CheckCompose(); err != nil {
		return err
	}

	ui.StepProgress("preflight", "Checking cloudflared")
	if err := cloudflared.CheckInstalled(); err != nil {
		return err
	}

	state.Paths = paths
	return nil
}

func runGitHubIdentity(ctx context.Context, state *State, ui UI) error {
	clientID := strings.TrimSpace(os.Getenv("GUNGNR_GITHUB_CLIENT_ID"))
	if clientID == "" {
		value, err := ui.Prompt(ctx, Prompt{
			Label:    "GitHub OAuth Client ID for device flow",
			Help:     clistrings.GitHubDeviceFlowHelp(),
			Validate: func(value string) error { return validate.NonEmpty("GitHub client ID", value) },
		})
		if err != nil {
			return err
		}
		clientID = value
	}
	state.GitHubClientID = clientID

	ui.StepProgress("github_identity", "Requesting device code")
	deviceCode, err := github.RequestDeviceCode(clientID)
	if err != nil {
		return err
	}

	ui.Info("Authorize this machine with GitHub:")
	ui.Info("- Visit " + deviceCode.VerificationURI)
	ui.Info("- Enter code: " + deviceCode.UserCode)
	if deviceCode.VerificationURIComplete != "" {
		ui.Info("- Or open: " + deviceCode.VerificationURIComplete)
	}

	ui.StepProgress("github_identity", "Waiting for authorization")
	token, err := github.PollAccessToken(clientID, deviceCode)
	if err != nil {
		return err
	}

	ui.StepProgress("github_identity", "Fetching GitHub profile")
	user, err := github.FetchUser(token.AccessToken)
	if err != nil {
		return err
	}
	state.GitHubUser = user
	ui.Info(fmt.Sprintf("Captured GitHub identity: %s (ID %d)", user.Login, user.ID))
	return nil
}

func runCloudflaredTunnel(ctx context.Context, state *State, ui UI) error {
	ui.StepProgress("cloudflared_tunnel", "Running cloudflared tunnel login")
	if err := cloudflared.Login(); err != nil {
		return fmt.Errorf("cloudflared tunnel login failed: %w", err)
	}

	ui.StepProgress("cloudflared_tunnel", "Waiting for Cloudflare credentials")
	if _, err := cloudflared.WaitForOriginCert(state.Paths.CloudflaredDir, 2*time.Minute); err != nil {
		return fmt.Errorf("cloudflared credentials not found after login: %w", err)
	}

	tunnelName, err := ui.Prompt(ctx, Prompt{
		Label:    "Tunnel name",
		Validate: func(value string) error { return validate.NonEmpty("tunnel name", value) },
	})
	if err != nil {
		return err
	}

	ui.StepProgress("cloudflared_tunnel", "Creating tunnel")
	tunnel, err := cloudflared.CreateTunnel(state.Paths.CloudflaredDir, tunnelName)
	if err != nil {
		return err
	}
	state.Tunnel = tunnel
	ui.Info(fmt.Sprintf("Cloudflare tunnel ready: %s (UUID %s)", tunnel.Name, tunnel.ID))
	return nil
}

func runCloudflareDNS(ctx context.Context, state *State, ui UI) error {
	if state.Tunnel == nil {
		return errors.New("cloudflared tunnel details missing before DNS setup")
	}
	baseDomain, err := ui.Prompt(ctx, Prompt{
		Label:     "Base domain (example.com)",
		Validate:  validate.Domain,
		Normalize: validate.NormalizeDomain,
	})
	if err != nil {
		return err
	}

	apiToken, err := ui.Prompt(ctx, Prompt{
		Label:     "Cloudflare API token (paste token only)",
		Help:      clistrings.CloudflareTokenHelp(),
		Secret:    true,
		Validate:  func(value string) error { return validate.NonEmpty("Cloudflare API token", value) },
		Normalize: validate.NormalizeCloudflareToken,
	})
	if err != nil {
		return err
	}

	ui.StepProgress("cloudflare_dns", "Validating zone access")
	zone, err := cloudflare.FetchZone(apiToken, baseDomain)
	if err != nil {
		return err
	}

	accountName, err := cloudflare.VerifyAccountAccess(apiToken, zone.Account.ID)
	if err != nil {
		return err
	}
	ui.Info(fmt.Sprintf("Validated zone %s (account %s).", zone.Name, accountName))

	hostname := fmt.Sprintf("panel.%s", baseDomain)
	ui.StepProgress("cloudflare_dns", fmt.Sprintf("Creating DNS route for %s", hostname))
	if err := cloudflared.RouteDNS(state.Tunnel.ID, hostname); err != nil {
		return err
	}

	ui.StepProgress("cloudflare_dns", "Verifying DNS record")
	if err := cloudflare.VerifyDNSRecord(apiToken, zone.ID, hostname); err != nil {
		return err
	}

	state.DNS = &cloudflareSetup{
		BaseDomain:  baseDomain,
		Hostname:    hostname,
		APIToken:    apiToken,
		ZoneID:      zone.ID,
		AccountID:   zone.Account.ID,
		AccountName: accountName,
	}
	state.PanelHostname = hostname
	state.PanelURL = "https://" + hostname
	ui.Info(fmt.Sprintf("DNS routing confirmed for %s", hostname))
	return nil
}

func runCloudflaredRun(ctx context.Context, state *State, ui UI) error {
	if state.Tunnel == nil {
		return errors.New("cloudflared tunnel details missing before config step")
	}
	if state.PanelHostname == "" {
		return errors.New("panel hostname missing before config step")
	}
	ui.StepProgress("cloudflared_run", "Writing cloudflared config")
	configPath, err := cloudflared.WriteConfig(state.Paths.CloudflaredDir, state.Tunnel, state.PanelHostname)
	if err != nil {
		return err
	}
	state.CloudflaredConfig = configPath

	ui.StepProgress("cloudflared_run", "Starting cloudflared tunnel")
	logPath, err := cloudflared.StartTunnel(configPath)
	if err != nil {
		return err
	}
	state.CloudflaredLogPath = logPath
	ui.Info("Cloudflared tunnel logs: " + logPath)

	ui.StepProgress("cloudflared_run", "Waiting for tunnel health")
	if err := cloudflared.WaitForRunning(state.Tunnel.ID, 2*time.Minute); err != nil {
		return err
	}

	ui.Info("Cloudflared tunnel is running.")
	ui.Info(clistrings.CloudflaredPersistenceNote())
	return nil
}

func runEnvSetup(ctx context.Context, state *State, ui UI) error {
	if state.GitHubUser == nil {
		return errors.New("GitHub identity missing before env generation")
	}
	if state.DNS == nil {
		return errors.New("Cloudflare DNS details missing before env generation")
	}
	if state.Tunnel == nil {
		return errors.New("cloudflared tunnel details missing before env generation")
	}
	ui.StepProgress("env_setup", "Preparing data directories")
	dataPaths, err := filesystem.PrepareDataDir(state.Paths.DataDir)
	if err != nil {
		return err
	}
	state.DataPaths = dataPaths

	ui.StepProgress("env_setup", "Wiring tunnel auto-start")
	autoStart, err := cloudflared.SetupAutoStart(state.CloudflaredConfig, state.DataPaths.StateDir)
	if err != nil {
		return err
	}
	state.CloudflaredAutoStart = autoStart
	if autoStart.CronInstalled {
		ui.Info("Tunnel auto-start: " + autoStart.CronDetail)
		ui.Info("Tunnel ensure script: " + autoStart.EnsureScript)
	} else if strings.TrimSpace(autoStart.CronDetail) != "" {
		ui.Warn("Tunnel auto-start: " + autoStart.CronDetail)
	}

	githubSecret, err := ui.Prompt(ctx, Prompt{
		Label:    "GitHub OAuth Client Secret",
		Secret:   true,
		Validate: func(value string) error { return validate.NonEmpty("GitHub client secret", value) },
	})
	if err != nil {
		return err
	}
	state.GitHubClientSecret = githubSecret

	callbackDefault := fmt.Sprintf("https://%s/auth/callback", state.PanelHostname)
	callbackURL, err := ui.Prompt(ctx, Prompt{
		Label:    "GitHub OAuth Callback URL",
		Default:  callbackDefault,
		Validate: func(value string) error { return validate.NonEmpty("GitHub callback URL", value) },
	})
	if err != nil {
		return err
	}
	state.GitHubCallbackURL = callbackURL

	sessionSecret, err := GenerateSessionSecret(32)
	if err != nil {
		return err
	}

	state.Env = BootstrapEnv{
		AppEnv:              "prod",
		Port:                "8080",
		DatabaseURL:         BuildDatabaseURL(DefaultPostgresUser, DefaultPostgresPassword, DefaultPostgresDB),
		DBMaxOpenConns:      20,
		DBMaxIdleConns:      10,
		DBConnMaxLifetime:   30,
		CORSAllowedOrigins:  BuildCORSOrigins(state.PanelHostname),
		SessionSecret:       sessionSecret,
		SessionTTLHours:     12,
		CookieDomain:        state.DNS.BaseDomain,
		GitHubClientID:      state.GitHubClientID,
		GitHubClientSecret:  state.GitHubClientSecret,
		GitHubCallbackURL:   state.GitHubCallbackURL,
		GitHubTemplateOwner: "Hrafngud",
		GitHubTemplateRepo:  "go-ground",
		GitHubRepoPrivate:   true,
		SuperUserGitHubName: state.GitHubUser.Login,
		SuperUserGitHubID:   state.GitHubUser.ID,
		TemplatesDir:        dataPaths.TemplatesDir,
		Domain:              state.DNS.BaseDomain,
		CloudflareAPIToken:  state.DNS.APIToken,
		CloudflareAccountID: state.DNS.AccountID,
		CloudflareZoneID:    state.DNS.ZoneID,
		CloudflareTunnelID:  state.Tunnel.ID,
		CloudflaredConfig:   state.CloudflaredConfig,
		CloudflaredTunnel:   state.Tunnel.Name,
		CloudflaredDir:      state.Paths.CloudflaredDir,
		PostgresUser:        DefaultPostgresUser,
		PostgresPassword:    DefaultPostgresPassword,
		PostgresDB:          DefaultPostgresDB,
		ViteAPIBaseURL:      "/",
	}

	if err := state.Env.Validate(); err != nil {
		return err
	}

	ui.StepProgress("env_setup", "Writing .env file")
	if err := filesystem.WriteEnvFile(dataPaths.EnvPath, state.Env.Entries()); err != nil {
		return err
	}
	return nil
}

func runComposeStart(ctx context.Context, state *State, ui UI) error {
	ui.StepProgress("compose_start", "Locating docker-compose.yml")
	composeFile, err := docker.FindComposeFile()
	if err != nil {
		return err
	}
	state.ComposeFile = composeFile

	ui.StepProgress("compose_start", "Starting Docker Compose services")
	logPath := filepath.Join(state.DataPaths.StateDir, "docker-compose.log")
	state.ComposeLogPath = logPath
	if err := docker.StartCompose(composeFile, state.DataPaths.EnvPath, logPath); err != nil {
		return err
	}

	ui.StepProgress("compose_start", "Waiting for API health check")
	if err := health.WaitForHTTPHealth("http://localhost/healthz", 3*time.Minute); err != nil {
		return err
	}

	ui.Info("Panel is ready: " + state.PanelURL)
	return nil
}
