package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/panelapi"
)

const (
	defaultPanelAPIBaseURL      = "http://localhost"
	netBirdModeApplySummaryLine = "netbird_mode_apply_summary="
)

type NetBirdModeSwitchOptions struct {
	TargetMode        string
	AllowLocalhost    bool
	PanelAPIBaseURL   string
	PanelAuthToken    string
	NetBirdAPIBaseURL string
	NetBirdAPIToken   string
	HostPeerID        string
	AdminPeerIDs      []string
	AutoApprove       bool
	PollInterval      time.Duration
	PollTimeout       time.Duration
	Stdin             *os.File
	Stdout            io.Writer
}

type netBirdPlanOperationCounts struct {
	Create         int
	Update         int
	Delete         int
	DisableDefault int
	Other          int
}

type netBirdReconcileCounts struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

type netBirdExecutionCounts struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type netBirdExecutionSummary struct {
	Counts netBirdExecutionCounts `json:"counts"`
}

type netBirdModeApplySummary struct {
	GroupResultCounts  netBirdReconcileCounts  `json:"groupResultCounts"`
	PolicyResultCounts netBirdReconcileCounts  `json:"policyResultCounts"`
	RebindingExecution netBirdExecutionSummary `json:"rebindingExecution"`
	RedeployExecution  netBirdExecutionSummary `json:"redeployExecution"`
	Warnings           []string                `json:"warnings"`
}

func NetBirdModeSwitch(ctx context.Context, options NetBirdModeSwitchOptions) error {
	targetMode, err := normalizeNetBirdMode(options.TargetMode)
	if err != nil {
		return err
	}

	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	reader := bufio.NewReader(stdin)

	panelBaseURL, err := resolvePanelAPIBaseURL(options.PanelAPIBaseURL)
	if err != nil {
		return err
	}
	client, err := panelapi.NewClient(panelBaseURL)
	if err != nil {
		return err
	}

	panelAuthToken, authSource, err := resolvePanelAuthToken(ctx, client, strings.TrimSpace(options.PanelAuthToken))
	if err != nil {
		return err
	}
	client.SetAuthToken(panelAuthToken)

	fmt.Fprintf(stdout, "Panel API: %s\n", panelBaseURL)
	fmt.Fprintf(stdout, "Panel auth: %s\n", authSource)
	fmt.Fprintf(stdout, "Requesting dry-run plan for target mode %q...\n", targetMode)

	plan, err := client.PlanNetBirdMode(ctx, targetMode, options.AllowLocalhost)
	if err != nil {
		return fmt.Errorf("netbird mode plan request failed: %w", err)
	}
	printNetBirdPlanSummary(stdout, plan)

	applyRequest, err := buildNetBirdModeApplyRequest(targetMode, options, reader, stdin, stdout)
	if err != nil {
		return err
	}

	if !options.AutoApprove {
		confirmation, err := promptLine(reader, stdout, "Type 'apply' to continue (anything else aborts): ", false)
		if err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(confirmation)) != "apply" {
			return errors.New("aborted by user")
		}
	}

	job, err := client.ApplyNetBirdMode(ctx, applyRequest)
	if err != nil {
		return fmt.Errorf("netbird mode apply request failed: %w", err)
	}

	fmt.Fprintf(stdout, "Apply job queued: id=%d type=%s status=%s\n", job.ID, strings.TrimSpace(job.Type), strings.TrimSpace(job.Status))

	terminalJob, summary, err := pollNetBirdApplyJob(ctx, client, job.ID, options.PollInterval, options.PollTimeout, stdout)
	if err != nil {
		return err
	}

	printNetBirdApplyTerminalSummary(stdout, terminalJob, summary)
	outcome := classifyNetBirdApplyOutcome(terminalJob.Status, summary)
	switch outcome {
	case "completed_success":
		return nil
	case "completed_with_failures":
		return fmt.Errorf("netbird mode apply completed with failures (job %d)", terminalJob.ID)
	default:
		return fmt.Errorf("netbird mode apply failed (job %d)", terminalJob.ID)
	}
}

func normalizeNetBirdMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "legacy", "mode_a", "mode_b":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid target mode %q (expected legacy|mode_a|mode_b)", raw)
	}
}

func resolvePanelAPIBaseURL(explicit string) (string, error) {
	if value := strings.TrimSpace(explicit); value != "" {
		return panelapi.NormalizeBaseURL(value)
	}
	if value := strings.TrimSpace(os.Getenv("GUNGNR_PANEL_URL")); value != "" {
		return panelapi.NormalizeBaseURL(value)
	}

	paths, err := filesystem.DefaultPaths()
	if err == nil {
		envPath := filepath.Join(paths.DataDir, ".env")
		env := readEnvFile(envPath)
		if value := strings.TrimSpace(env["GUNGNR_PANEL_URL"]); value != "" {
			return panelapi.NormalizeBaseURL(value)
		}
	}

	return panelapi.NormalizeBaseURL(defaultPanelAPIBaseURL)
}

func resolvePanelAuthToken(ctx context.Context, client *panelapi.Client, explicitToken string) (token string, source string, err error) {
	if token = strings.TrimSpace(explicitToken); token != "" {
		return token, "flag", nil
	}
	if token = strings.TrimSpace(os.Getenv("GUNGNR_API_TOKEN")); token != "" {
		return token, "env:GUNGNR_API_TOKEN", nil
	}

	paths, pathErr := filesystem.DefaultPaths()
	if pathErr == nil {
		envPath := filepath.Join(paths.DataDir, ".env")
		env := readEnvFile(envPath)
		login := strings.TrimSpace(env["ADMIN_LOGIN"])
		password := strings.TrimSpace(env["ADMIN_PASSWORD"])
		if login != "" && password != "" {
			response, issueErr := client.IssueTestToken(ctx, login, password)
			if issueErr == nil {
				issued := strings.TrimSpace(response.Token)
				if issued != "" {
					return issued, "test-token:~/.gungnr/.env", nil
				}
			}
		}
	}

	return "", "", errors.New("panel auth token is required; pass --auth-token, set GUNGNR_API_TOKEN, or configure ADMIN_LOGIN/ADMIN_PASSWORD so CLI can call /test-token")
}

func buildNetBirdModeApplyRequest(
	targetMode string,
	options NetBirdModeSwitchOptions,
	reader *bufio.Reader,
	stdin *os.File,
	stdout io.Writer,
) (panelapi.NetBirdModeApplyRequest, error) {
	request := panelapi.NetBirdModeApplyRequest{
		TargetMode:     targetMode,
		AllowLocalhost: options.AllowLocalhost,
		APIBaseURL:     strings.TrimSpace(options.NetBirdAPIBaseURL),
		APIToken:       strings.TrimSpace(options.NetBirdAPIToken),
		HostPeerID:     strings.TrimSpace(options.HostPeerID),
		AdminPeerIDs:   normalizeStringList(options.AdminPeerIDs),
	}

	if request.APIBaseURL == "" {
		request.APIBaseURL = strings.TrimSpace(os.Getenv("NETBIRD_API_BASE_URL"))
	}
	if request.APIToken == "" {
		request.APIToken = strings.TrimSpace(os.Getenv("NETBIRD_API_TOKEN"))
	}
	if request.HostPeerID == "" {
		request.HostPeerID = strings.TrimSpace(os.Getenv("NETBIRD_HOST_PEER_ID"))
	}
	if len(request.AdminPeerIDs) == 0 {
		request.AdminPeerIDs = parseCommaSeparatedList(os.Getenv("NETBIRD_ADMIN_PEER_IDS"))
	}

	if request.APIToken == "" {
		value, err := promptSecret(reader, stdin, stdout, "NetBird API token: ")
		if err != nil {
			return panelapi.NetBirdModeApplyRequest{}, err
		}
		request.APIToken = strings.TrimSpace(value)
	}
	if request.APIToken == "" {
		return panelapi.NetBirdModeApplyRequest{}, errors.New("netbird api token is required")
	}

	if targetMode == "legacy" {
		request.HostPeerID = ""
		request.AdminPeerIDs = nil
		return request, nil
	}

	if request.HostPeerID == "" {
		value, err := promptLine(reader, stdout, "NetBird host peer ID: ", true)
		if err != nil {
			return panelapi.NetBirdModeApplyRequest{}, err
		}
		request.HostPeerID = strings.TrimSpace(value)
	}
	if request.HostPeerID == "" {
		return panelapi.NetBirdModeApplyRequest{}, errors.New("host peer ID is required for non-legacy mode")
	}

	if len(request.AdminPeerIDs) == 0 {
		value, err := promptLine(reader, stdout, "NetBird admin peer IDs (comma-separated): ", true)
		if err != nil {
			return panelapi.NetBirdModeApplyRequest{}, err
		}
		request.AdminPeerIDs = parseCommaSeparatedList(value)
	}
	if len(request.AdminPeerIDs) == 0 {
		return panelapi.NetBirdModeApplyRequest{}, errors.New("at least one admin peer ID is required for non-legacy mode")
	}

	return request, nil
}

func promptLine(reader *bufio.Reader, stdout io.Writer, label string, required bool) (string, error) {
	if _, err := fmt.Fprint(stdout, label); err != nil {
		return "", err
	}

	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return "", errors.New("input is required")
	}
	return trimmed, nil
}

func promptSecret(reader *bufio.Reader, stdin *os.File, stdout io.Writer, label string) (string, error) {
	if _, err := fmt.Fprint(stdout, label); err != nil {
		return "", err
	}

	if stdin != nil && term.IsTerminal(int(stdin.Fd())) {
		value, err := term.ReadPassword(int(stdin.Fd()))
		if _, writeErr := fmt.Fprintln(stdout); writeErr != nil {
			return "", writeErr
		}
		if err != nil {
			return "", fmt.Errorf("read secret input: %w", err)
		}
		return strings.TrimSpace(string(value)), nil
	}

	return promptLine(reader, stdout, "", true)
}

func pollNetBirdApplyJob(
	ctx context.Context,
	client *panelapi.Client,
	jobID uint64,
	pollInterval time.Duration,
	pollTimeout time.Duration,
	stdout io.Writer,
) (panelapi.JobDetail, *netBirdModeApplySummary, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if pollTimeout <= 0 {
		pollTimeout = 30 * time.Minute
	}

	pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastStatus := ""
	logOffset := 0
	for {
		job, err := client.GetJob(pollCtx, jobID)
		if err != nil {
			return panelapi.JobDetail{}, nil, fmt.Errorf("failed to load job status: %w", err)
		}

		if status := strings.TrimSpace(strings.ToLower(job.Status)); status != lastStatus {
			fmt.Fprintf(stdout, "Job %d status: %s\n", jobID, status)
			lastStatus = status
		}
		logOffset = printJobLogDelta(stdout, job.LogLines, logOffset)

		switch strings.TrimSpace(strings.ToLower(job.Status)) {
		case "completed", "failed":
			summary, parseErr := parseNetBirdModeApplySummary(job.LogLines)
			if parseErr != nil {
				return job, nil, fmt.Errorf("failed to parse netbird apply summary payload: %w", parseErr)
			}
			return job, summary, nil
		}

		select {
		case <-pollCtx.Done():
			return panelapi.JobDetail{}, nil, fmt.Errorf("job polling timed out after %s", pollTimeout)
		case <-ticker.C:
		}
	}
}

func printJobLogDelta(stdout io.Writer, lines []string, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines) {
		offset = len(lines)
	}

	for i := offset; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, netBirdModeApplySummaryLine) {
			continue
		}
		fmt.Fprintf(stdout, "job> %s\n", line)
	}
	return len(lines)
}

func parseNetBirdModeApplySummary(lines []string) (*netBirdModeApplySummary, error) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, netBirdModeApplySummaryLine) {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, netBirdModeApplySummaryLine))
		if payload == "" {
			return nil, nil
		}
		var summary netBirdModeApplySummary
		if err := json.Unmarshal([]byte(payload), &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}
	return nil, nil
}

func classifyNetBirdApplyOutcome(status string, summary *netBirdModeApplySummary) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		if summary != nil && (summary.RebindingExecution.Counts.Failed > 0 || summary.RedeployExecution.Counts.Failed > 0) {
			return "completed_with_failures"
		}
		return "completed_success"
	case "failed":
		if summary != nil {
			return "completed_with_failures"
		}
		return "failed"
	default:
		return "failed"
	}
}

func printNetBirdPlanSummary(stdout io.Writer, plan panelapi.NetBirdModePlan) {
	groupCounts := countPlanOperations(plan.GroupOperations)
	policyCounts := countPlanOperations(plan.PolicyOperations)

	panelRebindings := 0
	projectRebindings := 0
	for _, op := range plan.ServiceRebindingOperations {
		switch strings.ToLower(strings.TrimSpace(op.Service)) {
		case "panel":
			panelRebindings++
		case "project_ingress":
			projectRebindings++
		}
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "NetBird Mode Plan")
	fmt.Fprintf(stdout, "  Current mode: %s\n", strings.TrimSpace(plan.CurrentMode))
	fmt.Fprintf(stdout, "  Target mode: %s\n", strings.TrimSpace(plan.TargetMode))
	fmt.Fprintf(stdout, "  Allow localhost: %t\n", plan.AllowLocalhost)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Operations")
	fmt.Fprintf(stdout, "  Group ops: create=%d update=%d delete=%d other=%d\n", groupCounts.Create, groupCounts.Update, groupCounts.Delete, groupCounts.Other)
	fmt.Fprintf(stdout, "  Policy ops: create=%d update=%d delete=%d disable-default=%d other=%d\n", policyCounts.Create, policyCounts.Update, policyCounts.Delete, policyCounts.DisableDefault, policyCounts.Other)
	fmt.Fprintf(stdout, "  Service rebindings: total=%d panel=%d project_ingress=%d\n", len(plan.ServiceRebindingOperations), panelRebindings, projectRebindings)
	fmt.Fprintf(stdout, "  Redeploy targets: panel=%t projects=%d\n", plan.RedeployTargets.Panel, len(plan.RedeployTargets.Projects))
	if len(plan.Warnings) == 0 {
		fmt.Fprintln(stdout, "  Warnings: none")
		return
	}
	fmt.Fprintf(stdout, "  Warnings (%d):\n", len(plan.Warnings))
	for _, warning := range plan.Warnings {
		text := strings.TrimSpace(warning)
		if text == "" {
			continue
		}
		fmt.Fprintf(stdout, "    - %s\n", text)
	}
}

func printNetBirdApplyTerminalSummary(stdout io.Writer, job panelapi.JobDetail, summary *netBirdModeApplySummary) {
	outcome := classifyNetBirdApplyOutcome(job.Status, summary)

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "NetBird Mode Apply Result")
	fmt.Fprintf(stdout, "  Job ID: %d\n", job.ID)
	fmt.Fprintf(stdout, "  Status: %s\n", strings.TrimSpace(job.Status))
	switch outcome {
	case "completed_success":
		fmt.Fprintln(stdout, "  Outcome: completed successfully")
	case "completed_with_failures":
		fmt.Fprintln(stdout, "  Outcome: completed with failures")
	default:
		fmt.Fprintln(stdout, "  Outcome: failed")
	}

	if msg := strings.TrimSpace(job.Error); msg != "" {
		fmt.Fprintf(stdout, "  Error: %s\n", msg)
	}

	if summary == nil {
		fmt.Fprintln(stdout, "  Summary payload: not found")
		return
	}

	fmt.Fprintln(stdout, "  Reconcile counts:")
	fmt.Fprintf(stdout, "    Groups: created=%d updated=%d deleted=%d unchanged=%d\n", summary.GroupResultCounts.Created, summary.GroupResultCounts.Updated, summary.GroupResultCounts.Deleted, summary.GroupResultCounts.Unchanged)
	fmt.Fprintf(stdout, "    Policies: created=%d updated=%d deleted=%d unchanged=%d\n", summary.PolicyResultCounts.Created, summary.PolicyResultCounts.Updated, summary.PolicyResultCounts.Deleted, summary.PolicyResultCounts.Unchanged)

	fmt.Fprintln(stdout, "  Execution counts:")
	fmt.Fprintf(stdout, "    Rebinding: succeeded=%d failed=%d skipped=%d\n", summary.RebindingExecution.Counts.Succeeded, summary.RebindingExecution.Counts.Failed, summary.RebindingExecution.Counts.Skipped)
	fmt.Fprintf(stdout, "    Redeploy: succeeded=%d failed=%d skipped=%d\n", summary.RedeployExecution.Counts.Succeeded, summary.RedeployExecution.Counts.Failed, summary.RedeployExecution.Counts.Skipped)

	if len(summary.Warnings) == 0 {
		fmt.Fprintln(stdout, "  Warnings: none")
		return
	}
	fmt.Fprintf(stdout, "  Warnings (%d):\n", len(summary.Warnings))
	for _, warning := range summary.Warnings {
		text := strings.TrimSpace(warning)
		if text == "" {
			continue
		}
		fmt.Fprintf(stdout, "    - %s\n", text)
	}
}

func countPlanOperations(operations []panelapi.NetBirdOperation) netBirdPlanOperationCounts {
	counts := netBirdPlanOperationCounts{}
	for _, operation := range operations {
		switch strings.ToLower(strings.TrimSpace(operation.Operation)) {
		case "create":
			counts.Create++
		case "update":
			counts.Update++
		case "delete":
			counts.Delete++
		case "disable-default":
			counts.DisableDefault++
		default:
			counts.Other++
		}
	}
	return counts
}

func parseCommaSeparatedList(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
