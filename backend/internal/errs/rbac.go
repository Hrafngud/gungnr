package errs

import "net/http"

var (
	CodeRBACSuperUserCap = RegisterHTTPStatus("RBAC-409-SUPERUSER-CAP", http.StatusConflict)
)
