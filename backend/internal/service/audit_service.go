package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const defaultAuditLimit = 100

type AuditEntry struct {
	UserID    uint
	UserLogin string
	Action    string
	Target    string
	Metadata  any
}

type AuditService struct {
	repo repository.AuditLogRepository
}

func NewAuditService(repo repository.AuditLogRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) List(ctx context.Context, limit int) ([]models.AuditLog, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultAuditLimit
	}
	return s.repo.List(ctx, limit)
}

func (s *AuditService) Log(ctx context.Context, entry AuditEntry) error {
	if s == nil || s.repo == nil {
		return nil
	}
	action := strings.TrimSpace(entry.Action)
	if action == "" {
		return fmt.Errorf("audit action is empty")
	}

	metadata, err := marshalMetadata(entry.Metadata)
	if err != nil {
		return err
	}

	record := models.AuditLog{
		UserID:    entry.UserID,
		UserLogin: strings.TrimSpace(entry.UserLogin),
		Action:    action,
		Target:    strings.TrimSpace(entry.Target),
		Metadata:  metadata,
	}
	return s.repo.Create(ctx, &record)
}

func marshalMetadata(metadata any) (string, error) {
	if metadata == nil {
		return "", nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal audit metadata: %w", err)
	}
	trimmed := strings.TrimSpace(string(encoded))
	if trimmed == "null" {
		return "", nil
	}
	return trimmed, nil
}
