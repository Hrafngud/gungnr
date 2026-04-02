package errs

import "net/http"

var (
	CodeGitHubUnavailable  = RegisterHTTPStatus("GH-500-SERVICE", http.StatusInternalServerError)
	CodeGitHubCatalog      = RegisterHTTPStatus("GH-500-CATALOG", http.StatusInternalServerError)
	CodeGitHubMissingToken = RegisterHTTPStatus("GH-400-TOKEN", http.StatusBadRequest)
)
