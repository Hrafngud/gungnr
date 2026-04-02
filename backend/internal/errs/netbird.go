package errs

import "net/http"

var (
	CodeNetBirdUnavailable     = RegisterHTTPStatus("NETBIRD-500-SERVICE", http.StatusInternalServerError)
	CodeNetBirdStatusFailed    = RegisterHTTPStatus("NETBIRD-500-STATUS", http.StatusInternalServerError)
	CodeNetBirdACLGraphFailed  = RegisterHTTPStatus("NETBIRD-500-ACL-GRAPH", http.StatusInternalServerError)
	CodeNetBirdPlanFailed      = RegisterHTTPStatus("NETBIRD-500-PLAN", http.StatusInternalServerError)
	CodeNetBirdApplyFailed     = RegisterHTTPStatus("NETBIRD-500-APPLY", http.StatusInternalServerError)
	CodeNetBirdReapplyFailed   = RegisterHTTPStatus("NETBIRD-500-REAPPLY", http.StatusInternalServerError)
	CodeNetBirdReconcileFailed = RegisterHTTPStatus("NETBIRD-500-RECONCILE", http.StatusInternalServerError)
	CodeNetBirdInvalidBody     = RegisterHTTPStatus("NETBIRD-400-BODY", http.StatusBadRequest)
	CodeNetBirdInvalidMode     = RegisterHTTPStatus("NETBIRD-400-MODE", http.StatusBadRequest)
	CodeNetBirdAdminRequired   = RegisterHTTPStatus("NETBIRD-403-ADMIN", http.StatusForbidden)
)
