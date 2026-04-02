package errs

import "net/http"

var (
	CodeInternal         = RegisterHTTPStatus("CORE-500", http.StatusInternalServerError)
	CodeBadRequest       = RegisterHTTPStatus("CORE-400", http.StatusBadRequest)
	CodeNotFound         = RegisterHTTPStatus("CORE-404", http.StatusNotFound)
	CodeValidationFields = RegisterHTTPStatus("VAL-400-FIELDS", http.StatusBadRequest)
	CodeValidationName   = RegisterHTTPStatus("VAL-400-NAME", http.StatusBadRequest)
	CodeValidationSubdomain = RegisterHTTPStatus("VAL-400-SUBDOMAIN", http.StatusBadRequest)
	CodeValidationPort      = RegisterHTTPStatus("VAL-400-PORT", http.StatusBadRequest)
	CodeValidationDomain    = RegisterHTTPStatus("VAL-400-DOMAIN", http.StatusBadRequest)
	CodeDomainMissing       = RegisterHTTPStatus("VAL-400-DOMAIN-MISSING", http.StatusBadRequest)
	CodeDomainNotConfigured = RegisterHTTPStatus("VAL-400-DOMAIN-NOT-CONFIGURED", http.StatusBadRequest)
	CodeContainerName       = RegisterHTTPStatus("VAL-400-CONTAINER-NAME", http.StatusBadRequest)
)
