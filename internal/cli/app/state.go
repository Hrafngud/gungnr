package app

import (
	"gungnr-cli/internal/cli/integrations/cloudflared"
	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/github"
)

type State struct {
	Paths              filesystem.Paths
	DataPaths          filesystem.DataPaths
	GitHubClientID     string
	GitHubClientSecret string
	GitHubCallbackURL  string
	GitHubUser         *github.User
	Tunnel             *cloudflared.TunnelInfo
	DNS                *cloudflareSetup
	CloudflaredConfig  string
	CloudflaredLogPath string
	Env                BootstrapEnv
	ComposeFile        string
	ComposeLogPath     string
	PanelHostname      string
	PanelURL           string
}

type cloudflareSetup struct {
	BaseDomain  string
	Hostname    string
	APIToken    string
	ZoneID      string
	AccountID   string
	AccountName string
}

type Summary struct {
	DataDir             string
	TemplatesDir        string
	StateDir            string
	EnvPath             string
	PanelURL            string
	CloudflaredConfig   string
	CloudflaredLog      string
	ComposeLog          string
	CloudflaredTunnel   string
	CloudflaredTunnelID string
}

func (s State) Summary() Summary {
	return Summary{
		DataDir:             s.DataPaths.Root,
		TemplatesDir:        s.DataPaths.TemplatesDir,
		StateDir:            s.DataPaths.StateDir,
		EnvPath:             s.DataPaths.EnvPath,
		PanelURL:            s.PanelURL,
		CloudflaredConfig:   s.CloudflaredConfig,
		CloudflaredLog:      s.CloudflaredLogPath,
		ComposeLog:          s.ComposeLogPath,
		CloudflaredTunnel:   s.Env.CloudflaredTunnel,
		CloudflaredTunnelID: s.Env.CloudflareTunnelID,
	}
}
