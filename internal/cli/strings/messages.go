package clistrings

const (
	GitHubOAuthAppURL   = "https://github.com/settings/developers"
	GitHubDeviceFlowDoc = "https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app"
	CloudflareTokenURL  = "https://dash.cloudflare.com/profile/api-tokens"
	CloudflareTokenDoc  = "https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/deployment-guides/terraform/#3-create-a-cloudflare-api-token"
)

func GitHubDeviceFlowHelp() []string {
	return []string{
		"GitHub device flow uses an OAuth App client ID.",
		"Create one at: " + GitHubOAuthAppURL,
		"Enable Device Flow in the app settings (required): " + GitHubDeviceFlowDoc,
	}
}

func CloudflareTokenHelp() []string {
	return []string{
		"Create a Cloudflare API token with Account: Cloudflare Tunnel: Edit and Zone: DNS: Edit.",
		"Token page: " + CloudflareTokenURL,
		"Docs: " + CloudflareTokenDoc,
	}
}

func CloudflaredPersistenceNote() string {
	return "Note: keepalive installs a system-level systemd reboot-recovery timer that rebuilds the panel stack, restarts the tunnel, and then rebuilds project stacks."
}

func KeepaliveBootstrapHelp() []string {
	return []string{
		"Reboot keepalive runs after host startup to recover panel and project stacks.",
		"Answer yes to install system-level keepalive during bootstrap, or no to skip it.",
	}
}
