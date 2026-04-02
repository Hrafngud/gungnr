package errs

import "net/http"

var (
	CodeAuthUnauthenticated    = RegisterHTTPStatus("AUTH-401", http.StatusUnauthorized)
	CodeAuthForbidden          = RegisterHTTPStatus("AUTH-403", http.StatusForbidden)
	CodeAuthAdminRequired      = RegisterHTTPStatus("AUTH-403-ADMIN", http.StatusForbidden)
	CodeAuthStateGenerate      = RegisterHTTPStatus("AUTH-500-STATE", http.StatusInternalServerError)
	CodeAuthCallbackMissing    = RegisterHTTPStatus("AUTH-400-CALLBACK", http.StatusBadRequest)
	CodeAuthStateInvalid       = RegisterHTTPStatus("AUTH-400-STATE", http.StatusBadRequest)
	CodeAuthLoginFailed        = RegisterHTTPStatus("AUTH-500-LOGIN", http.StatusInternalServerError)
	CodeAuthSessionCreate      = RegisterHTTPStatus("AUTH-500-SESSION", http.StatusInternalServerError)
	CodeAuthTestTokenInvalid   = RegisterHTTPStatus("AUTH-400-TEST-TOKEN", http.StatusBadRequest)
	CodeAuthTestTokenDisabled  = RegisterHTTPStatus("AUTH-404-TEST-TOKEN", http.StatusNotFound)
	CodeAuthInvalidCredentials = RegisterHTTPStatus("AUTH-401-CREDENTIALS", http.StatusUnauthorized)
)
