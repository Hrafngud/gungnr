package errs

import "net/http"

var (
	CodeAuditListFailed = RegisterHTTPStatus("AUDIT-500-LIST", http.StatusInternalServerError)
)
