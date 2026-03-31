package errs

import "net/http"

var (
	CodeUserInvalidID       = RegisterHTTPStatus("USER-400-ID", http.StatusBadRequest)
	CodeUserInvalidPayload  = RegisterHTTPStatus("USER-400-PAYLOAD", http.StatusBadRequest)
	CodeUserInvalidRole     = RegisterHTTPStatus("USER-400-ROLE", http.StatusBadRequest)
	CodeUserLoginRequired   = RegisterHTTPStatus("USER-400-LOGIN", http.StatusBadRequest)
	CodeUserNotFound        = RegisterHTTPStatus("USER-404", http.StatusNotFound)
	CodeUserLastSuperUser   = RegisterHTTPStatus("USER-400-LAST-SUPERUSER", http.StatusBadRequest)
	CodeUserGitHubNotFound  = RegisterHTTPStatus("USER-404-GITHUB", http.StatusNotFound)
	CodeUserRemoveSuperUser = RegisterHTTPStatus("USER-400-SUPERUSER", http.StatusBadRequest)
	CodeUserListFailed      = RegisterHTTPStatus("USER-500-LIST", http.StatusInternalServerError)
	CodeUserUpdateFailed    = RegisterHTTPStatus("USER-500-UPDATE", http.StatusInternalServerError)
	CodeUserCreateFailed    = RegisterHTTPStatus("USER-500-CREATE", http.StatusInternalServerError)
	CodeUserDeleteFailed    = RegisterHTTPStatus("USER-500-DELETE", http.StatusInternalServerError)
)
