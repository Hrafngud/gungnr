package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"
)

const (
	DefaultPollInterval = 500 * time.Millisecond
	DefaultWaitTimeout  = 2 * time.Minute
)

type TimeoutError struct {
	IntentID string
	Timeout  time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("infra bridge wait timed out for intent %s after %s", e.IntentID, e.Timeout)
}

type TaskFailedError struct {
	IntentID string
	Code     string
	Message  string
	LogPath  string
}

func (e *TaskFailedError) Error() string {
	if strings.TrimSpace(e.Code) != "" {
		return fmt.Sprintf("infra bridge task failed for intent %s (%s): %s", e.IntentID, e.Code, e.Message)
	}
	return fmt.Sprintf("infra bridge task failed for intent %s: %s", e.IntentID, e.Message)
}

type Client struct {
	queue        *queue.Filesystem
	pollInterval time.Duration
	waitTimeout  time.Duration
}

func New(q *queue.Filesystem, pollInterval, waitTimeout time.Duration) *Client {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	if waitTimeout <= 0 {
		waitTimeout = DefaultWaitTimeout
	}
	return &Client{
		queue:        q,
		pollInterval: pollInterval,
		waitTimeout:  waitTimeout,
	}
}

func (c *Client) SubmitIntent(ctx context.Context, requestID string, taskType contract.TaskType, payload map[string]any) (contract.Intent, error) {
	if c == nil || c.queue == nil {
		return contract.Intent{}, fmt.Errorf("infra bridge queue is unavailable")
	}
	intentID, err := newIntentID()
	if err != nil {
		return contract.Intent{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = intentID
	}
	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  intentID,
		RequestID: requestID,
		TaskType:  taskType,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := c.queue.WriteIntent(ctx, intent); err != nil {
		return contract.Intent{}, fmt.Errorf("submit intent %s: %w", intentID, err)
	}
	return intent, nil
}

func (c *Client) WaitResult(ctx context.Context, intentID string) (contract.Result, error) {
	if c == nil || c.queue == nil {
		return contract.Result{}, fmt.Errorf("infra bridge queue is unavailable")
	}
	waitCtx := ctx
	cancel := func() {}
	if c.waitTimeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, c.waitTimeout)
	}
	defer cancel()

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		result, err := c.LoadResult(waitCtx, intentID)
		if err == nil {
			if result.Terminal() {
				return result, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return contract.Result{}, err
		}

		select {
		case <-waitCtx.Done():
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return contract.Result{}, ctx.Err()
				}
				return contract.Result{}, &TimeoutError{IntentID: intentID, Timeout: c.waitTimeout}
			}
			return contract.Result{}, waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) LoadResult(ctx context.Context, intentID string) (contract.Result, error) {
	if c == nil || c.queue == nil {
		return contract.Result{}, fmt.Errorf("infra bridge queue is unavailable")
	}
	result, err := c.queue.ReadResult(ctx, intentID)
	if err != nil {
		return contract.Result{}, fmt.Errorf("load result %s: %w", intentID, err)
	}
	return result, nil
}

func (c *Client) RestartTunnel(ctx context.Context, requestID, configPath string) (contract.Result, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return contract.Result{}, fmt.Errorf("cloudflared config path is required")
	}
	return c.runTask(ctx, requestID, contract.TaskTypeRestartTunnel, map[string]any{
		"config_path": configPath,
	})
}

func (c *Client) StopContainer(ctx context.Context, requestID, container string) (contract.Result, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return contract.Result{}, fmt.Errorf("container is required")
	}
	return c.runTask(ctx, requestID, contract.TaskTypeDockerStopContainer, map[string]any{
		"container": container,
	})
}

func (c *Client) RestartContainer(ctx context.Context, requestID, container string) (contract.Result, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return contract.Result{}, fmt.Errorf("container is required")
	}
	return c.runTask(ctx, requestID, contract.TaskTypeDockerRestartContainer, map[string]any{
		"container": container,
	})
}

func (c *Client) RemoveContainer(ctx context.Context, requestID, container string, removeVolumes bool) (contract.Result, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return contract.Result{}, fmt.Errorf("container is required")
	}
	payload := map[string]any{
		"container": container,
	}
	if removeVolumes {
		payload["remove_volumes"] = true
	}
	return c.runTask(ctx, requestID, contract.TaskTypeDockerRemoveContainer, payload)
}

func (c *Client) ComposeUpStack(ctx context.Context, requestID string, payload contract.ComposeUpStackPayload) (contract.Result, error) {
	payload.Project = strings.TrimSpace(payload.Project)
	payload.ProjectDir = strings.TrimSpace(payload.ProjectDir)
	if payload.Project == "" {
		return contract.Result{}, fmt.Errorf("project is required")
	}

	intentPayload := map[string]any{
		"project": payload.Project,
	}
	if payload.ProjectDir != "" {
		intentPayload["project_dir"] = payload.ProjectDir
	}
	if len(payload.ConfigFiles) > 0 {
		intentPayload["config_files"] = payload.ConfigFiles
	}
	if payload.Build {
		intentPayload["build"] = true
	}
	if payload.ForceRecreate {
		intentPayload["force_recreate"] = true
	}

	return c.runTask(ctx, requestID, contract.TaskTypeComposeUpStack, intentPayload)
}

func (c *Client) runTask(ctx context.Context, requestID string, taskType contract.TaskType, payload map[string]any) (contract.Result, error) {
	intent, err := c.SubmitIntent(ctx, requestID, taskType, payload)
	if err != nil {
		return contract.Result{}, err
	}
	result, err := c.WaitResult(ctx, intent.IntentID)
	if err != nil {
		return contract.Result{}, err
	}
	if result.Status == contract.StatusFailed {
		return result, toTaskFailedError(result)
	}
	return result, nil
}

func toTaskFailedError(result contract.Result) error {
	failed := &TaskFailedError{
		IntentID: result.IntentID,
		LogPath:  result.LogPath,
	}
	if result.Error != nil {
		failed.Code = result.Error.Code
		failed.Message = result.Error.Message
	}
	if strings.TrimSpace(failed.Message) == "" {
		failed.Message = "host worker reported failure"
	}
	return failed
}

func newIntentID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate intent id: %w", err)
	}
	return "intent-" + hex.EncodeToString(buf), nil
}
