package models

// StopJobRequest is the request body for stopping a job.
type StopJobRequest struct {
	Error string `json:"error"`
}

// JobDetailResponse extends JobResponse with log lines.
type JobDetailResponse struct {
	JobResponse
	LogLines []string `json:"logLines"`
}
