package models

// CloudflareZoneResponse is the API response shape for a Cloudflare zone.
type CloudflareZoneResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
