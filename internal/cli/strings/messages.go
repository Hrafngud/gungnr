package clistrings

const (
	GitHubOAuthAppURL  = "https://github.com/settings/developers"
	CloudflareTokenURL = "https://dash.cloudflare.com/profile/api-tokens"
	CloudflareTokenDoc = "https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/deployment-guides/terraform/#3-create-a-cloudflare-api-token"
)

func GitHubDeviceFlowHelp() []string {
	return []string{
		"GitHub device flow uses an OAuth App client ID.",
		"Create one at: " + GitHubOAuthAppURL,
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
