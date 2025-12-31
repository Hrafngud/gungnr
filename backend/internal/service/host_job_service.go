package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const (
	JobStatusPendingHost = "pending_host"
)

var (
	ErrHostTokenInvalid = errors.New("host token invalid")
	ErrHostTokenExpired = errors.New("host token expired")
	ErrHostTokenUsed    = errors.New("host token already used")
)

type HostDeployRequest struct {
	JobType string          `json:"jobType"`
	Payload json.RawMessage `json:"payload"`
}

type HostDeployPayload struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type HostJobService struct {
	repo     repository.JobRepository
	tokenTTL time.Duration
}

func NewHostJobService(repo repository.JobRepository, tokenTTL time.Duration) *HostJobService {
	if tokenTTL <= 0 {
		tokenTTL = 30 * time.Minute
	}
	return &HostJobService{repo: repo, tokenTTL: tokenTTL}
}

func (s *HostJobService) CreateHostDeploy(ctx context.Context, req HostDeployRequest) (*models.Job, string, *time.Time, error) {
	jobType := strings.TrimSpace(req.JobType)
	if jobType == "" {
		return nil, "", nil, fmt.Errorf("job type is required")
	}
	normalized, err := normalizeHostPayload(jobType, req.Payload)
	if err != nil {
		return nil, "", nil, err
	}

	payload := HostDeployPayload{
		Action:  jobType,
		Payload: normalized,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", nil, fmt.Errorf("marshal host payload: %w", err)
	}

	token, err := generateHostToken()
	if err != nil {
		return nil, "", nil, err
	}
	expiresAt := time.Now().Add(s.tokenTTL)

	job := models.Job{
		Type:               JobTypeHostDeploy,
		Status:             JobStatusPendingHost,
		Input:              string(body),
		HostToken:          token,
		HostTokenExpiresAt: &expiresAt,
	}
	if err := s.repo.Create(ctx, &job); err != nil {
		return nil, "", nil, fmt.Errorf("create host job: %w", err)
	}

	return &job, token, &expiresAt, nil
}

func (s *HostJobService) ClaimJob(ctx context.Context, token string) (*models.Job, HostDeployPayload, error) {
	job, payload, err := s.loadJobByToken(ctx, token)
	if err != nil {
		return nil, HostDeployPayload{}, err
	}

	if job.Status == JobStatusPendingHost {
		now := time.Now()
		if err := s.repo.MarkRunning(ctx, job.ID, now); err != nil {
			return nil, HostDeployPayload{}, err
		}
		if err := s.repo.MarkHostTokenClaimed(ctx, job.ID, now); err != nil {
			return nil, HostDeployPayload{}, err
		}
		job.Status = "running"
		job.StartedAt = &now
	}

	return job, payload, nil
}

func (s *HostJobService) AppendLogs(ctx context.Context, token string, lines []string) (*models.Job, error) {
	job, _, err := s.ClaimJob(ctx, token)
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		entry := fmt.Sprintf("%s\n", trimmed)
		if err := s.repo.AppendLog(ctx, job.ID, entry); err != nil {
			return nil, err
		}
	}

	return job, nil
}

func (s *HostJobService) CompleteJob(ctx context.Context, token string, status string, errMsg string) (*models.Job, error) {
	job, _, err := s.loadJobByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	status = strings.TrimSpace(status)
	if status == "" {
		if strings.TrimSpace(errMsg) != "" {
			status = "failed"
		} else {
			status = "completed"
		}
	}
	if status != "completed" && status != "failed" {
		return nil, fmt.Errorf("invalid status")
	}

	finishedAt := time.Now()
	if err := s.repo.MarkFinished(ctx, job.ID, status, finishedAt, strings.TrimSpace(errMsg)); err != nil {
		return nil, err
	}
	if err := s.repo.MarkHostTokenUsed(ctx, job.ID, finishedAt); err != nil {
		return nil, err
	}

	job.Status = status
	job.FinishedAt = &finishedAt
	job.Error = strings.TrimSpace(errMsg)
	job.HostTokenUsedAt = &finishedAt

	return job, nil
}

func (s *HostJobService) loadJobByToken(ctx context.Context, token string) (*models.Job, HostDeployPayload, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, HostDeployPayload{}, ErrHostTokenInvalid
	}

	job, err := s.repo.GetByHostToken(ctx, trimmed)
	if err != nil {
		return nil, HostDeployPayload{}, err
	}
	if job.Type != JobTypeHostDeploy {
		return nil, HostDeployPayload{}, ErrHostTokenInvalid
	}
	if job.HostTokenUsedAt != nil {
		return nil, HostDeployPayload{}, ErrHostTokenUsed
	}
	if job.HostTokenExpiresAt != nil && time.Now().After(*job.HostTokenExpiresAt) {
		return nil, HostDeployPayload{}, ErrHostTokenExpired
	}

	payload, err := parseHostDeployPayload(job.Input)
	if err != nil {
		return nil, HostDeployPayload{}, err
	}
	return job, payload, nil
}

func parseHostDeployPayload(raw string) (HostDeployPayload, error) {
	var payload HostDeployPayload
	if strings.TrimSpace(raw) == "" {
		return payload, fmt.Errorf("host payload empty")
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, fmt.Errorf("parse host payload: %w", err)
	}
	if strings.TrimSpace(payload.Action) == "" {
		return payload, fmt.Errorf("host payload missing action")
	}
	if len(payload.Payload) == 0 {
		return payload, fmt.Errorf("host payload missing request")
	}
	return payload, nil
}

func normalizeHostPayload(jobType string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("payload is required")
	}

	switch jobType {
	case JobTypeCreateTemplate:
		var req CreateTemplateRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("parse payload: %w", err)
		}
		req.Name = strings.ToLower(strings.TrimSpace(req.Name))
		req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
		if req.Subdomain == "" {
			req.Subdomain = req.Name
		}
		if err := ValidateProjectName(req.Name); err != nil {
			return nil, err
		}
		if err := ValidateSubdomain(req.Subdomain); err != nil {
			return nil, err
		}
		if req.ProxyPort != 0 {
			if err := ValidatePort(req.ProxyPort); err != nil {
				return nil, err
			}
		}
		if req.DBPort != 0 {
			if err := ValidatePort(req.DBPort); err != nil {
				return nil, err
			}
		}
		return json.Marshal(req)
	case JobTypeDeployExisting:
		var req DeployExistingRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("parse payload: %w", err)
		}
		req.Name = strings.ToLower(strings.TrimSpace(req.Name))
		req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
		if err := ValidateProjectName(req.Name); err != nil {
			return nil, err
		}
		if err := ValidateSubdomain(req.Subdomain); err != nil {
			return nil, err
		}
		if req.Port == 0 {
			req.Port = 80
		}
		if err := ValidatePort(req.Port); err != nil {
			return nil, err
		}
		return json.Marshal(req)
	case JobTypeQuickService:
		var req QuickServiceRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("parse payload: %w", err)
		}
		req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
		if err := ValidateSubdomain(req.Subdomain); err != nil {
			return nil, err
		}
		if err := ValidatePort(req.Port); err != nil {
			return nil, err
		}
		return json.Marshal(req)
	default:
		return nil, fmt.Errorf("unsupported job type")
	}
}

func generateHostToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
