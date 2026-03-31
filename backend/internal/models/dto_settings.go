package models

// SettingsFullResponse is the API response for settings with metadata.
// Note: uses any for settings/sources to avoid circular import with service package.
// The controller constructs this with the concrete service types.
type SettingsFullResponse struct {
	Settings              any    `json:"settings"`
	Sources               any    `json:"sources,omitempty"`
	CloudflaredTunnelName string `json:"cloudflaredTunnelName,omitempty"`
	TemplatesDir          string `json:"templatesDir,omitempty"`
}
