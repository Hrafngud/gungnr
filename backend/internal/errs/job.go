package errs

import "net/http"

var (
	CodeJobInvalidID         = RegisterHTTPStatus("JOB-400-ID", http.StatusBadRequest)
	CodeJobInvalidBody       = RegisterHTTPStatus("JOB-400-BODY", http.StatusBadRequest)
	CodeJobNotFound          = RegisterHTTPStatus("JOB-404", http.StatusNotFound)
	CodeJobAlreadyFinished   = RegisterHTTPStatus("JOB-409-FINISHED", http.StatusConflict)
	CodeJobRunning           = RegisterHTTPStatus("JOB-409-RUNNING", http.StatusConflict)
	CodeJobNotStoppable      = RegisterHTTPStatus("JOB-409-NOT-STOPPABLE", http.StatusConflict)
	CodeJobNotRetryable      = RegisterHTTPStatus("JOB-409-NOT-RETRYABLE", http.StatusConflict)
	CodeJobListFailed        = RegisterHTTPStatus("JOB-500-LIST", http.StatusInternalServerError)
	CodeJobStopFailed        = RegisterHTTPStatus("JOB-500-STOP", http.StatusInternalServerError)
	CodeJobRetryFailed       = RegisterHTTPStatus("JOB-500-RETRY", http.StatusInternalServerError)
	CodeJobStreamUnsupported = RegisterHTTPStatus("JOB-500-STREAM", http.StatusInternalServerError)
)
