package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-notes/internal/errs"
	netbirdapi "go-notes/internal/integrations/netbird"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const (
	netBirdSyncStatusNever     = "never"
	netBirdSyncStatusPending   = "pending"
	netBirdSyncStatusSucceeded = "succeeded"
	netBirdSyncStatusFailed    = "failed"
	netBirdSyncStatusUnknown   = "unknown"
)

const (
	netBirdManagedCountSourceCatalog = "catalog"
	netBirdManagedCountSourceAPI     = "api"
)

const (
	netBirdReachabilitySourceCatalog  = "catalog"
	netBirdReachabilitySourceLastSync = "last_sync"
	netBirdReachabilitySourceLive     = "live"
)

const netBirdModeApplySummaryPrefix = "netbird_mode_apply_summary="
const netBirdDaemonRecentHeartbeatWindow = 5 * time.Minute

type NetBirdStatus struct {
	ClientInstalled           bool                   `json:"clientInstalled"`
	DaemonRunning             bool                   `json:"daemonRunning"`
	Connected                 bool                   `json:"connected"`
	PeerID                    string                 `json:"peerId,omitempty"`
	PeerName                  string                 `json:"peerName,omitempty"`
	WG0IP                     string                 `json:"wg0Ip,omitempty"`
	CurrentMode               NetBirdMode            `json:"currentMode"`
	ConfiguredMode            NetBirdMode            `json:"configuredMode"`
	EffectiveModeBProjectIDs  []uint                 `json:"effectiveModeBProjectIds"`
	ConfiguredModeBProjectIDs []uint                 `json:"configuredModeBProjectIds"`
	ModeSource                string                 `json:"modeSource"`
	ModeSourceJobID           uint                   `json:"modeSourceJobId,omitempty"`
	ModeDrift                 bool                   `json:"modeDrift"`
	LastPolicySyncAt          *time.Time             `json:"lastPolicySyncAt,omitempty"`
	LastPolicySyncStatus      string                 `json:"lastPolicySyncStatus"`
	LastPolicySyncJobID       uint                   `json:"lastPolicySyncJobId,omitempty"`
	LastPolicySyncError       string                 `json:"lastPolicySyncError,omitempty"`
	LastPolicySyncWarning     int                    `json:"lastPolicySyncWarnings"`
	LastGroupResults          NetBirdOperationCounts `json:"lastGroupResults"`
	LastPolicyResults         NetBirdOperationCounts `json:"lastPolicyResults"`
	APIReachable              bool                   `json:"apiReachable"`
	APIReachability           NetBirdAPIReachability `json:"apiReachability"`
	ManagedGroups             int                    `json:"managedGroups"`
	ManagedPolicies           int                    `json:"managedPolicies"`
	ManagedCountSource        string                 `json:"managedCountSource"`
	Warnings                  []string               `json:"warnings"`
}

type NetBirdAPIReachability struct {
	Source    string     `json:"source"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type NetBirdACLGraph struct {
	CurrentMode               NetBirdMode      `json:"currentMode"`
	ConfiguredMode            NetBirdMode      `json:"configuredMode"`
	EffectiveModeBProjectIDs  []uint           `json:"effectiveModeBProjectIds"`
	ConfiguredModeBProjectIDs []uint           `json:"configuredModeBProjectIds"`
	ModeSource                string           `json:"modeSource"`
	ModeSourceJobID           uint             `json:"modeSourceJobId,omitempty"`
	ModeDrift                 bool             `json:"modeDrift"`
	DefaultAction             string           `json:"defaultAction"`
	Nodes                     []NetBirdACLNode `json:"nodes"`
	Edges                     []NetBirdACLEdge `json:"edges"`
	Notes                     []string         `json:"notes"`
}

type NetBirdACLNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	GroupName   string `json:"groupName,omitempty"`
	ProjectID   uint   `json:"projectId,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
}

type NetBirdACLEdge struct {
	ID            string   `json:"id"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Policy        string   `json:"policy"`
	Rule          string   `json:"rule"`
	Action        string   `json:"action"`
	Protocol      string   `json:"protocol"`
	Ports         []string `json:"ports"`
	Bidirectional bool     `json:"bidirectional"`
}

type netBirdModeApplySnapshot struct {
	Found             bool
	Job               models.Job
	Request           NetBirdModeApplyJobRequest
	RequestParsed     bool
	RequestParseError error
	Summary           *NetBirdModeApplySummary
	SummaryParseError error
}

type netBirdLiveStatus struct {
	ManagedGroups   int
	ManagedPolicies int
	GroupsKnown     bool
	PoliciesKnown   bool
	ClientInstalled bool
	DaemonRunning   bool
	Connected       bool
	PeerID          string
	PeerName        string
	WG0IP           string
	Warnings        []string
}

func (s *NetBirdService) Status(ctx context.Context) (NetBirdStatus, error) {
	if s == nil || s.projects == nil {
		return NetBirdStatus{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	runtimeState := s.resolveNetBirdRuntimeState(ctx)
	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdStatus{}, errs.Wrap(errs.CodeNetBirdStatusFailed, "failed to load netbird status project catalog", err)
	}
	runtimeModeProjects := []NetBirdProjectCatalogInput{}
	runtimeProjectWarnings := []string{}
	if runtimeState.EffectiveMode == NetBirdModeB {
		runtimeModeProjects, runtimeProjectWarnings = selectModeBProjects(projectInputs, runtimeState.EffectiveModeBProjectIDs)
	}
	catalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      runtimeState.EffectiveMode,
		PanelPort: panelPort,
		Projects:  runtimeModeProjects,
	})

	status := NetBirdStatus{
		CurrentMode:               runtimeState.EffectiveMode,
		ConfiguredMode:            runtimeState.ConfiguredMode,
		EffectiveModeBProjectIDs:  normalizeUintList(runtimeState.EffectiveModeBProjectIDs),
		ConfiguredModeBProjectIDs: normalizeUintList(runtimeState.ConfiguredModeBProjectIDs),
		ModeSource:                runtimeState.Source,
		ModeSourceJobID:           runtimeState.SourceJobID,
		ModeDrift:                 runtimeState.Drift,
		LastPolicySyncStatus:      netBirdSyncStatusNever,
		APIReachability: NetBirdAPIReachability{
			Source: netBirdReachabilitySourceCatalog,
		},
		ManagedGroups:      len(catalog.Groups),
		ManagedPolicies:    len(catalog.Policies),
		ManagedCountSource: netBirdManagedCountSourceCatalog,
		Warnings:           []string{},
	}

	status.Warnings = append(status.Warnings, runtimeState.Warnings...)
	if panelPortFallback {
		status.Warnings = append(status.Warnings, "Panel port was not a valid integer; status used default port 8080.")
	}
	status.Warnings = append(status.Warnings, projectWarnings...)
	status.Warnings = append(status.Warnings, runtimeProjectWarnings...)
	if runtimeState.EffectiveMode == NetBirdModeB && len(runtimeState.EffectiveModeBProjectIDs) == 0 {
		status.Warnings = append(status.Warnings, "Mode B is active with no assigned projects; only panel isolation policies are managed.")
	}

	snapshot, err := s.latestModeApplySnapshot(ctx)
	if err != nil {
		return NetBirdStatus{}, errs.Wrap(errs.CodeNetBirdStatusFailed, "failed to load latest netbird apply job", err)
	}
	if !snapshot.Found {
		status.APIReachability.Message = "No NetBird mode apply job has been recorded yet."
		return status, nil
	}

	status.LastPolicySyncJobID = snapshot.Job.ID
	status.LastPolicySyncStatus = syncStatusFromJob(snapshot.Job.Status)
	status.LastPolicySyncError = strings.TrimSpace(snapshot.Job.Error)
	if snapshot.Job.FinishedAt != nil {
		status.LastPolicySyncAt = cloneTimePtr(snapshot.Job.FinishedAt)
	}

	if snapshot.Summary != nil {
		if !snapshot.Summary.CompletedAt.IsZero() {
			status.LastPolicySyncAt = cloneTimePtr(&snapshot.Summary.CompletedAt)
		}
		status.LastPolicySyncWarning = len(snapshot.Summary.Warnings)
		status.LastGroupResults = snapshot.Summary.GroupResultCounts
		status.LastPolicyResults = snapshot.Summary.PolicyResultCounts
	}
	if snapshot.SummaryParseError != nil {
		status.Warnings = append(status.Warnings, "Latest NetBird apply job summary could not be parsed; fallback sync fields were used.")
	}
	if snapshot.RequestParseError != nil {
		status.Warnings = append(status.Warnings, "Latest NetBird apply job request could not be parsed; live API status fallback may be incomplete.")
	}

	resolvedRequest := NetBirdModeApplyRequest{}
	if snapshot.RequestParsed {
		resolvedRequest = NetBirdModeApplyRequest{
			TargetMode:     snapshot.Request.TargetMode,
			AllowLocalhost: snapshot.Request.AllowLocalhost,
			APIBaseURL:     snapshot.Request.APIBaseURL,
			APIToken:       snapshot.Request.APIToken,
			HostPeerID:     snapshot.Request.HostPeerID,
			AdminPeerIDs:   append([]string(nil), snapshot.Request.AdminPeerIDs...),
		}
	}
	if s.settings != nil {
		mergedRequest, _, err := s.settings.ResolveNetBirdModeApplyRequest(ctx, resolvedRequest)
		if err != nil {
			status.Warnings = append(status.Warnings, fmt.Sprintf("Failed to resolve saved NetBird mode config for live status: %v", err))
		} else {
			resolvedRequest = mergedRequest
		}
	}

	apiToken := strings.TrimSpace(resolvedRequest.APIToken)
	apiBaseURL := strings.TrimSpace(resolvedRequest.APIBaseURL)
	hostPeerID := strings.TrimSpace(resolvedRequest.HostPeerID)
	if hostPeerID != "" {
		status.ClientInstalled = true
		if status.LastPolicySyncStatus == netBirdSyncStatusSucceeded {
			status.DaemonRunning = true
			status.Connected = true
		}
	}
	if apiToken == "" {
		status.APIReachability.Source = netBirdReachabilitySourceLastSync
		status.APIReachable = status.LastPolicySyncStatus == netBirdSyncStatusSucceeded
		if !status.APIReachable && status.APIReachability.Message == "" {
			status.APIReachability.Message = "No API token was available from saved NetBird mode config or latest mode apply context."
		}
		if status.ClientInstalled || status.DaemonRunning || status.Connected {
			status.Warnings = append(status.Warnings, "Live NetBird API credentials were unavailable; connectivity indicators reflect the latest successful sync snapshot.")
		}
		return status, nil
	}

	liveCheckedAt := time.Now().UTC()
	status.APIReachability.Source = netBirdReachabilitySourceLive
	status.APIReachability.CheckedAt = &liveCheckedAt
	live, err := fetchNetBirdLiveStatus(ctx, apiBaseURL, apiToken, hostPeerID)
	if err != nil {
		status.APIReachability.Message = fmt.Sprintf("Live NetBird API check failed: %v", err)
		restoredFromLastSuccess := false
		if isNetBirdLiveAuthFailure(err) {
			lastSuccessSnapshot, snapshotErr := s.latestSuccessfulModeApplySnapshot(ctx)
			if snapshotErr != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("Failed to load last successful NetBird sync snapshot: %v", snapshotErr))
			} else {
				restoredFromLastSuccess = applyLastKnownNetBirdConnectivity(&status, lastSuccessSnapshot)
			}
		}
		if status.ClientInstalled || status.DaemonRunning || status.Connected {
			if restoredFromLastSuccess {
				status.Warnings = append(status.Warnings, "Live NetBird API authentication failed; connectivity indicators are using last known successful sync state.")
			} else {
				status.Warnings = append(status.Warnings, "Live NetBird API check failed; connectivity indicators currently reflect the latest successful sync snapshot.")
			}
		}
		return status, nil
	}

	status.APIReachable = true
	if live.GroupsKnown {
		status.ManagedGroups = live.ManagedGroups
	}
	if live.PoliciesKnown {
		status.ManagedPolicies = live.ManagedPolicies
	}
	if live.GroupsKnown && live.PoliciesKnown {
		status.ManagedCountSource = netBirdManagedCountSourceAPI
	}
	status.ClientInstalled = live.ClientInstalled
	status.DaemonRunning = live.DaemonRunning
	status.Connected = live.Connected
	status.PeerID = live.PeerID
	status.PeerName = live.PeerName
	status.WG0IP = live.WG0IP
	status.Warnings = append(status.Warnings, live.Warnings...)
	if len(live.Warnings) > 0 {
		status.APIReachability.Message = strings.Join(live.Warnings, " | ")
	}

	return status, nil
}

func (s *NetBirdService) ACLGraph(ctx context.Context) (NetBirdACLGraph, error) {
	if s == nil || s.projects == nil {
		return NetBirdACLGraph{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	runtimeState := s.resolveNetBirdRuntimeState(ctx)
	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdACLGraph{}, errs.Wrap(errs.CodeNetBirdACLGraphFailed, "failed to load netbird acl graph project catalog", err)
	}
	runtimeModeProjects := []NetBirdProjectCatalogInput{}
	runtimeProjectWarnings := []string{}
	if runtimeState.EffectiveMode == NetBirdModeB {
		runtimeModeProjects, runtimeProjectWarnings = selectModeBProjects(projectInputs, runtimeState.EffectiveModeBProjectIDs)
	}
	catalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      runtimeState.EffectiveMode,
		PanelPort: panelPort,
		Projects:  runtimeModeProjects,
	})

	graph := buildNetBirdACLGraph(runtimeState.EffectiveMode, catalog, runtimeModeProjects)
	graph.ConfiguredMode = runtimeState.ConfiguredMode
	graph.EffectiveModeBProjectIDs = normalizeUintList(runtimeState.EffectiveModeBProjectIDs)
	graph.ConfiguredModeBProjectIDs = normalizeUintList(runtimeState.ConfiguredModeBProjectIDs)
	graph.ModeSource = runtimeState.Source
	graph.ModeSourceJobID = runtimeState.SourceJobID
	graph.ModeDrift = runtimeState.Drift
	graph.Notes = append(graph.Notes, runtimeState.Warnings...)
	if panelPortFallback {
		graph.Notes = append(graph.Notes, "Panel port was not a valid integer; ACL graph used default port 8080.")
	}
	graph.Notes = append(graph.Notes, projectWarnings...)
	graph.Notes = append(graph.Notes, runtimeProjectWarnings...)
	if runtimeState.EffectiveMode == NetBirdModeB && len(runtimeState.EffectiveModeBProjectIDs) == 0 {
		graph.Notes = append(graph.Notes, "Mode B is active with no assigned projects; ACL graph includes panel-only policies.")
	}
	if runtimeState.EffectiveMode == NetBirdModeLegacy {
		graph.Notes = append(graph.Notes, "Legacy mode has no managed NetBird ACL edges.")
	}

	return graph, nil
}

func configuredNetBirdMode(raw string) (NetBirdMode, bool) {
	normalized := normalizeMode(raw)
	switch normalized {
	case NetBirdModeLegacy:
		return NetBirdModeLegacy, true
	case NetBirdModeA:
		return NetBirdModeA, true
	case NetBirdModeB:
		return NetBirdModeB, true
	default:
		return NetBirdModeLegacy, false
	}
}

func (s *NetBirdService) latestModeApplySnapshot(ctx context.Context) (netBirdModeApplySnapshot, error) {
	result := netBirdModeApplySnapshot{}
	if s == nil || s.jobs == nil {
		return result, nil
	}

	job, err := s.jobs.GetLatestByType(ctx, JobTypeNetBirdModeApply)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return result, nil
		}
		return result, err
	}
	if job == nil {
		return result, nil
	}

	return parseModeApplySnapshot(*job), nil
}

func (s *NetBirdService) latestSuccessfulModeApplySnapshot(ctx context.Context) (netBirdModeApplySnapshot, error) {
	result := netBirdModeApplySnapshot{}
	if s == nil || s.jobs == nil {
		return result, nil
	}

	job, err := s.jobs.GetLatestByTypeAndStatus(ctx, JobTypeNetBirdModeApply, "completed")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return result, nil
		}
		return result, err
	}
	if job == nil {
		return result, nil
	}

	return parseModeApplySnapshot(*job), nil
}

func parseModeApplySnapshot(job models.Job) netBirdModeApplySnapshot {
	result := netBirdModeApplySnapshot{
		Found: true,
		Job:   job,
	}

	trimmedInput := strings.TrimSpace(job.Input)
	if trimmedInput != "" {
		var request NetBirdModeApplyJobRequest
		if err := json.Unmarshal([]byte(trimmedInput), &request); err == nil {
			result.Request = normalizeNetBirdModeApplyJobRequest(request)
			result.RequestParsed = true
		} else {
			result.RequestParseError = err
		}
	}

	summary, err := parseNetBirdModeApplySummary(job.LogLines)
	if err != nil {
		result.SummaryParseError = err
		return result
	}
	result.Summary = summary
	return result
}

func applyLastKnownNetBirdConnectivity(status *NetBirdStatus, snapshot netBirdModeApplySnapshot) bool {
	if status == nil || !snapshot.Found || !snapshot.RequestParsed {
		return false
	}
	hostPeerID := strings.TrimSpace(snapshot.Request.HostPeerID)
	if hostPeerID == "" {
		return false
	}
	status.ClientInstalled = true
	status.DaemonRunning = true
	status.Connected = true
	if strings.TrimSpace(status.PeerID) == "" {
		status.PeerID = hostPeerID
	}
	return true
}

func isNetBirdLiveAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	return strings.Contains(message, "invalid token") ||
		strings.Contains(message, "status=401") ||
		strings.Contains(message, "status=403") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "forbidden")
}

func parseNetBirdModeApplySummary(logLines string) (*NetBirdModeApplySummary, error) {
	trimmed := strings.TrimSpace(logLines)
	if trimmed == "" {
		return nil, nil
	}

	lines := strings.Split(trimmed, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, netBirdModeApplySummaryPrefix) {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, netBirdModeApplySummaryPrefix))
		if payload == "" {
			return nil, nil
		}
		var summary NetBirdModeApplySummary
		if err := json.Unmarshal([]byte(payload), &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}
	return nil, nil
}

func fetchNetBirdLiveStatus(ctx context.Context, apiBaseURL, apiToken, hostPeerID string) (netBirdLiveStatus, error) {
	client := netbirdapi.NewClient(apiBaseURL, apiToken)
	peers, err := client.ListPeers(ctx)
	if err != nil {
		return netBirdLiveStatus{}, err
	}

	result := netBirdLiveStatus{
		ClientInstalled: strings.TrimSpace(hostPeerID) != "",
		Warnings:        []string{},
	}

	host := resolveHostPeer(peers, nil, hostPeerID)
	if host == nil {
		groups, groupsErr := client.ListGroups(ctx)
		if groupsErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Live NetBird groups check failed: %v", groupsErr))
		} else {
			result.ManagedGroups = countManagedGroups(groups)
			result.GroupsKnown = true
			host = resolveHostPeer(peers, groups, hostPeerID)
		}
	} else {
		groups, groupsErr := client.ListGroups(ctx)
		if groupsErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Live NetBird groups check failed: %v", groupsErr))
		} else {
			result.ManagedGroups = countManagedGroups(groups)
			result.GroupsKnown = true
		}
	}

	policies, policiesErr := client.ListPolicies(ctx)
	if policiesErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Live NetBird policies check failed: %v", policiesErr))
	} else {
		result.ManagedPolicies = countManagedPolicies(policies)
		result.PoliciesKnown = true
	}

	if host != nil {
		result.ClientInstalled = true
		result.Connected = host.Connected
		result.DaemonRunning = host.Connected
		result.PeerID = strings.TrimSpace(host.ID)
		result.PeerName = strings.TrimSpace(host.Name)
		result.WG0IP = strings.TrimSpace(host.IP)
		if !host.Connected {
			inferredDaemonRunning, warning := inferDaemonRunningFromPeer(*host, time.Now().UTC())
			result.DaemonRunning = inferredDaemonRunning
			if warning != "" {
				result.Warnings = append(result.Warnings, warning)
			}
		}
	} else if strings.TrimSpace(hostPeerID) != "" {
		result.Warnings = append(result.Warnings, "Host peer ID was not found in the live NetBird peer list.")
	}

	return result, nil
}

func inferDaemonRunningFromPeer(peer netbirdapi.Peer, now time.Time) (bool, string) {
	lastSeenRaw := strings.TrimSpace(peer.LastSeen)
	if lastSeenRaw == "" {
		return false, "Live peer is disconnected and has no heartbeat timestamp; daemon status assumed offline."
	}

	lastSeen, err := parseNetBirdTimestamp(lastSeenRaw)
	if err != nil {
		return false, "Live peer heartbeat timestamp could not be parsed; daemon status assumed offline."
	}

	age := now.Sub(lastSeen)
	if age <= netBirdDaemonRecentHeartbeatWindow {
		return true, fmt.Sprintf("Live peer is disconnected but heartbeat is recent (%s ago); daemon may still be running.", roundDuration(age))
	}

	return false, fmt.Sprintf("Live peer heartbeat is stale (%s ago); daemon status marked offline.", roundDuration(age))
}

func parseNetBirdTimestamp(raw string) (time.Time, error) {
	if value, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return value.UTC(), nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return value.UTC(), nil
}

func roundDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	if d < time.Second {
		return d
	}
	return d.Round(time.Second)
}

func countManagedGroups(groups []netbirdapi.Group) int {
	count := 0
	for _, group := range groups {
		name := strings.ToLower(strings.TrimSpace(group.Name))
		if strings.HasPrefix(name, "gungnr-") {
			count++
		}
	}
	return count
}

func countManagedPolicies(policies []netbirdapi.Policy) int {
	count := 0
	for _, policy := range policies {
		name := strings.ToLower(strings.TrimSpace(policy.Name))
		if strings.HasPrefix(name, "gungnr-") {
			count++
		}
	}
	return count
}

func resolveHostPeer(peers []netbirdapi.Peer, groups []netbirdapi.Group, hostPeerID string) *netbirdapi.Peer {
	byID := make(map[string]netbirdapi.Peer, len(peers))
	for _, peer := range peers {
		id := strings.TrimSpace(peer.ID)
		if id == "" {
			continue
		}
		byID[id] = peer
	}

	hostPeerID = strings.TrimSpace(hostPeerID)
	if hostPeerID != "" {
		if peer, ok := byID[hostPeerID]; ok {
			return &peer
		}
	}

	panelPeerID := ""
	for _, group := range groups {
		if strings.TrimSpace(group.Name) != netBirdGroupPanelName {
			continue
		}
		if len(group.Peers) == 0 {
			break
		}
		panelPeerID = strings.TrimSpace(group.Peers[0])
		break
	}
	if panelPeerID == "" {
		return nil
	}
	peer, ok := byID[panelPeerID]
	if !ok {
		return nil
	}
	return &peer
}

func syncStatusFromJob(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return netBirdSyncStatusSucceeded
	case "failed":
		return netBirdSyncStatusFailed
	case "running", "pending", "pending_host":
		return netBirdSyncStatusPending
	default:
		return netBirdSyncStatusUnknown
	}
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := *t
	return &value
}

func buildNetBirdACLGraph(mode NetBirdMode, catalog NetBirdCatalog, projects []NetBirdProjectCatalogInput) NetBirdACLGraph {
	nodes := make([]NetBirdACLNode, 0, len(projects)+2)
	nodeByID := make(map[string]NetBirdACLNode, len(projects)+2)
	placeholderToNode := map[string]string{}

	addNode := func(node NetBirdACLNode) {
		if node.ID == "" {
			return
		}
		if _, exists := nodeByID[node.ID]; exists {
			return
		}
		nodeByID[node.ID] = node
		nodes = append(nodes, node)
	}

	if mode == NetBirdModeA || mode == NetBirdModeB {
		admins := NetBirdACLNode{
			ID:        "group:admins",
			Label:     "Admins",
			Kind:      "group",
			GroupName: netBirdGroupAdminsName,
		}
		panel := NetBirdACLNode{
			ID:        "service:panel",
			Label:     "Panel",
			Kind:      "service",
			GroupName: netBirdGroupPanelName,
		}
		addNode(admins)
		addNode(panel)
		placeholderToNode[netBirdGroupIDAdmins] = admins.ID
		placeholderToNode[netBirdGroupIDPanel] = panel.ID
		placeholderToNode[netBirdGroupAdminsName] = admins.ID
		placeholderToNode[netBirdGroupPanelName] = panel.ID
	}

	for _, project := range projects {
		if mode != NetBirdModeB {
			break
		}
		label := strings.TrimSpace(project.ProjectName)
		if label == "" {
			label = fmt.Sprintf("Project %d", project.ProjectID)
		}
		node := NetBirdACLNode{
			ID:          projectNodeID(project.ProjectID),
			Label:       label,
			Kind:        "project",
			GroupName:   netBirdProjectGroupName(project.ProjectID),
			ProjectID:   project.ProjectID,
			ProjectName: strings.TrimSpace(project.ProjectName),
		}
		addNode(node)
		placeholderToNode[netBirdProjectGroupID(project.ProjectID)] = node.ID
		placeholderToNode[netBirdProjectGroupName(project.ProjectID)] = node.ID
	}

	edges := make([]NetBirdACLEdge, 0, len(catalog.Policies))
	for _, policy := range catalog.Policies {
		if !policy.Enabled {
			continue
		}
		for _, rule := range policy.Rules {
			if !rule.Enabled || strings.ToLower(strings.TrimSpace(rule.Action)) != netBirdActionAccept {
				continue
			}
			fromIDs := resolveGraphNodeIDs(rule.Sources, placeholderToNode)
			toIDs := resolveGraphNodeIDs(rule.Destinations, placeholderToNode)
			if len(fromIDs) == 0 || len(toIDs) == 0 {
				continue
			}
			ports := append([]string(nil), rule.Ports...)
			sort.Strings(ports)
			for _, fromID := range fromIDs {
				for _, toID := range toIDs {
					edgeID := fmt.Sprintf("%s:%s:%s:%s", strings.TrimSpace(policy.Name), strings.TrimSpace(rule.Name), fromID, toID)
					edges = append(edges, NetBirdACLEdge{
						ID:            edgeID,
						From:          fromID,
						To:            toID,
						Policy:        strings.TrimSpace(policy.Name),
						Rule:          strings.TrimSpace(rule.Name),
						Action:        strings.TrimSpace(rule.Action),
						Protocol:      strings.TrimSpace(rule.Protocol),
						Ports:         ports,
						Bidirectional: rule.Bidirectional,
					})
				}
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			if edges[i].To == edges[j].To {
				return edges[i].ID < edges[j].ID
			}
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	return NetBirdACLGraph{
		CurrentMode:   mode,
		DefaultAction: "deny",
		Nodes:         nodes,
		Edges:         edges,
		Notes:         []string{"All unspecified paths are blocked by the managed policy model."},
	}
}

func resolveGraphNodeIDs(rawValues []string, mapByPlaceholder map[string]string) []string {
	seen := map[string]struct{}{}
	resolved := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		nodeID, ok := mapByPlaceholder[key]
		if !ok {
			if projectID, parsed := parseProjectGroupPlaceholder(key); parsed {
				nodeID = projectNodeID(projectID)
				ok = true
			}
		}
		if !ok || nodeID == "" {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		resolved = append(resolved, nodeID)
	}
	sort.Strings(resolved)
	return resolved
}

func parseProjectGroupPlaceholder(raw string) (uint, bool) {
	trimmed := strings.TrimSpace(raw)
	const prefix = "${GROUP_ID_GUNGNR_PROJECT_"
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, "}") {
		return 0, false
	}
	segment := strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), "}")
	if segment == "" {
		return 0, false
	}
	value, err := strconv.ParseUint(segment, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(value), true
}

func projectNodeID(projectID uint) string {
	return fmt.Sprintf("project:%d", projectID)
}
