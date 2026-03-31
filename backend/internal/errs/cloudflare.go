package errs

import "net/http"

var (
	CodeCloudflareUnavailable    = RegisterHTTPStatus("CF-500-SERVICE", http.StatusInternalServerError)
	CodeCloudflarePreflight      = RegisterHTTPStatus("CF-502-PREFLIGHT", http.StatusBadGateway)
	CodeCloudflareZones          = RegisterHTTPStatus("CF-502-ZONES", http.StatusBadGateway)
	CodeCloudflareMissingToken   = RegisterHTTPStatus("CF-400-TOKEN", http.StatusBadRequest)
	CodeCloudflareMissingAccount = RegisterHTTPStatus("CF-400-ACCOUNT", http.StatusBadRequest)
	CodeCloudflareMissingZone    = RegisterHTTPStatus("CF-400-ZONE", http.StatusBadRequest)
	CodeCloudflareMissingTunnel  = RegisterHTTPStatus("CF-400-TUNNEL", http.StatusBadRequest)
	CodeCloudflareTunnelLocal    = RegisterHTTPStatus("CF-409-TUNNEL-LOCAL", http.StatusConflict)
)
