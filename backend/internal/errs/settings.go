package errs

import "net/http"

var (
	CodeSettingsLoadFailed    = RegisterHTTPStatus("SETTINGS-500-LOAD", http.StatusInternalServerError)
	CodeSettingsSourcesFailed = RegisterHTTPStatus("SETTINGS-500-SOURCES", http.StatusInternalServerError)
	CodeSettingsAdminRequired = RegisterHTTPStatus("SETTINGS-403-ADMIN", http.StatusForbidden)
	CodeSettingsInvalidBody   = RegisterHTTPStatus("SETTINGS-400-BODY", http.StatusBadRequest)
	CodeSettingsUpdateFailed  = RegisterHTTPStatus("SETTINGS-500-UPDATE", http.StatusInternalServerError)
	CodeSettingsPreviewFailed = RegisterHTTPStatus("SETTINGS-400-PREVIEW", http.StatusBadRequest)
	CodeSettingsSyncFailed    = RegisterHTTPStatus("SETTINGS-500-SYNC", http.StatusInternalServerError)
)
