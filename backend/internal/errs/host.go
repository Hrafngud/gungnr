package errs

import "net/http"

var (
	CodeHostInvalidProject    = RegisterHTTPStatus("HOST-400-PROJECT", http.StatusBadRequest)
	CodeHostInvalidContainer  = RegisterHTTPStatus("HOST-400-CONTAINER", http.StatusBadRequest)
	CodeHostInvalidBody       = RegisterHTTPStatus("HOST-400-BODY", http.StatusBadRequest)
	CodeHostAdminRequired     = RegisterHTTPStatus("HOST-403-ADMIN", http.StatusForbidden)
	CodeHostDockerFailed      = RegisterHTTPStatus("HOST-500-DOCKER", http.StatusInternalServerError)
	CodeHostUsageFailed       = RegisterHTTPStatus("HOST-500-USAGE", http.StatusInternalServerError)
	CodeHostStatsFailed       = RegisterHTTPStatus("HOST-500-STATS", http.StatusInternalServerError)
	CodeHostLogsFailed        = RegisterHTTPStatus("HOST-500-LOGS", http.StatusInternalServerError)
	CodeHostStreamUnsupported = RegisterHTTPStatus("HOST-500-STREAM", http.StatusInternalServerError)
)
