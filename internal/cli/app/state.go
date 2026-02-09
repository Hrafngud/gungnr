package app

import (
	"gungnr-cli/internal/cli/integrations/cloudflared"
	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/github"
)

type State struct {
	Paths                filesystem.Paths
	DataPaths            filesystem.DataPaths
	KeepaliveEnabled     bool
	KeepaliveStatus      string
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubCallbackURL    string
	GitHubUser           *github.User
	Tunnel               *cloudflared.TunnelInfo
	DNS                  *cloudflareSetup
	CloudflaredConfig    string
	CloudflaredLogPath   string
	CloudflaredAutoStart cloudflared.PersistenceResult
	Env                  BootstrapEnv
	ComposeFile          string
	ComposeLogPath       string
	PanelHostname        string
	PanelURL             string
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
	DataDir                 string
	TemplatesDir            string
	StateDir                string
	EnvPath                 string
	PanelURL                string
	KeepaliveStatus         string
	CloudflaredConfig       string
	CloudflaredLog          string
	ComposeLog              string
	CloudflaredTunnel       string
	CloudflaredTunnelID     string
	CloudflaredEnsureScript string
	CloudflaredCronDetail   string
}

func (s State) Summary() Summary {
	keepaliveStatus := s.KeepaliveStatus
	if keepaliveStatus == "" {
		if s.KeepaliveEnabled {
			keepaliveStatus = "enabled"
		} else {
			keepaliveStatus = "skipped"
		}
	}

	return Summary{
		DataDir:                 s.DataPaths.Root,
		TemplatesDir:            s.DataPaths.TemplatesDir,
		StateDir:                s.DataPaths.StateDir,
		EnvPath:                 s.DataPaths.EnvPath,
		PanelURL:                s.PanelURL,
		KeepaliveStatus:         keepaliveStatus,
		CloudflaredConfig:       s.CloudflaredConfig,
		CloudflaredLog:          s.CloudflaredLogPath,
		ComposeLog:              s.ComposeLogPath,
		CloudflaredTunnel:       s.Env.CloudflaredTunnel,
		CloudflaredTunnelID:     s.Env.CloudflareTunnelID,
		CloudflaredEnsureScript: s.CloudflaredAutoStart.EnsureScript,
		CloudflaredCronDetail:   s.CloudflaredAutoStart.Detail,
	}
}
