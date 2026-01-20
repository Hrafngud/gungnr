package clistrings

const (
	GitHubOAuthAppURL  = "https://github.com/settings/developers"
	GitHubDeviceFlowDoc = "https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app"
	CloudflareTokenURL = "https://dash.cloudflare.com/profile/api-tokens"
	CloudflareTokenDoc = "https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/deployment-guides/terraform/#3-create-a-cloudflare-api-token"
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
	return "Note: cloudflared runs as the current user; persistence and auto-restart are out of scope."
}
