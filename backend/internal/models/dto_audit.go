package models

import "time"

// AuditLogResponse is the API response shape for an audit log entry.
type AuditLogResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"userId"`
	UserLogin string    `json:"userLogin"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Metadata  string    `json:"metadata"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewAuditLogResponse builds an AuditLogResponse from an AuditLog model.
func NewAuditLogResponse(log AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:        log.ID,
		UserID:    log.UserID,
		UserLogin: log.UserLogin,
		Action:    log.Action,
		Target:    log.Target,
		Metadata:  log.Metadata,
		CreatedAt: log.CreatedAt,
	}
}

// NewAuditLogResponses builds a slice of AuditLogResponse from AuditLog models.
func NewAuditLogResponses(logs []AuditLog) []AuditLogResponse {
	response := make([]AuditLogResponse, 0, len(logs))
	for _, log := range logs {
		response = append(response, NewAuditLogResponse(log))
	}
	return response
}
