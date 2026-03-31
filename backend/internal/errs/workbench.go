package errs

import "net/http"

var (
	CodeWorkbenchSourceNotFound        = RegisterHTTPStatus("WB-404-SOURCE", http.StatusNotFound)
	CodeWorkbenchSourceInvalid         = RegisterHTTPStatus("WB-400-SOURCE", http.StatusBadRequest)
	CodeWorkbenchLocked                = RegisterHTTPStatus("WB-409-LOCKED", http.StatusConflict)
	CodeWorkbenchStaleRevision         = RegisterHTTPStatus("WB-409-STALE-REVISION", http.StatusConflict)
	CodeWorkbenchDriftDetected         = RegisterHTTPStatus("WB-409-DRIFT-DETECTED", http.StatusConflict)
	CodeWorkbenchValidationFailed      = RegisterHTTPStatus("WB-422-VALIDATION", http.StatusUnprocessableEntity)
	CodeWorkbenchGenerateFailed        = RegisterHTTPStatus("WB-500-GENERATE", http.StatusInternalServerError)
	CodeWorkbenchStorageFailed         = RegisterHTTPStatus("WB-500-STORAGE", http.StatusInternalServerError)
	CodeWorkbenchBackupNotFound        = RegisterHTTPStatus("WB-404-BACKUP", http.StatusNotFound)
	CodeWorkbenchBackupIntegrity       = RegisterHTTPStatus("WB-409-BACKUP-INTEGRITY", http.StatusConflict)
	CodeWorkbenchBackupWriteFailed     = RegisterHTTPStatus("WB-500-BACKUP-WRITE", http.StatusInternalServerError)
	CodeWorkbenchBackupRetentionFailed = RegisterHTTPStatus("WB-500-BACKUP-RETENTION", http.StatusInternalServerError)
	CodeWorkbenchRestoreFailed         = RegisterHTTPStatus("WB-500-RESTORE", http.StatusInternalServerError)
)
