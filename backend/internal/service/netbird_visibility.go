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

type NetBirdStatus struct {
	ClientInstalled       bool                   `json:"clientInstalled"`
	DaemonRunning         bool                   `json:"daemonRunning"`
	Connected             bool                   `json:"connected"`
	PeerID                string                 `json:"peerId,omitempty"`
	PeerName              string                 `json:"peerName,omitempty"`
	WG0IP                 string                 `json:"wg0Ip,omitempty"`
	CurrentMode           NetBirdMode            `json:"currentMode"`
	LastPolicySyncAt      *time.Time             `json:"lastPolicySyncAt,omitempty"`
	LastPolicySyncStatus  string                 `json:"lastPolicySyncStatus"`
	LastPolicySyncJobID   uint                   `json:"lastPolicySyncJobId,omitempty"`
	LastPolicySyncError   string                 `json:"lastPolicySyncError,omitempty"`
	LastPolicySyncWarning int                    `json:"lastPolicySyncWarnings"`
	LastGroupResults      NetBirdOperationCounts `json:"lastGroupResults"`
	LastPolicyResults     NetBirdOperationCounts `json:"lastPolicyResults"`
	APIReachable          bool                   `json:"apiReachable"`
	APIReachability       NetBirdAPIReachability `json:"apiReachability"`
	ManagedGroups         int                    `json:"managedGroups"`
	ManagedPolicies       int                    `json:"managedPolicies"`
	ManagedCountSource    string                 `json:"managedCountSource"`
	Warnings              []string               `json:"warnings"`
}

type NetBirdAPIReachability struct {
	Source    string     `json:"source"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type NetBirdACLGraph struct {
	CurrentMode   NetBirdMode      `json:"currentMode"`
	DefaultAction string           `json:"defaultAction"`
	Nodes         []NetBirdACLNode `json:"nodes"`
	Edges         []NetBirdACLEdge `json:"edges"`
	Notes         []string         `json:"notes"`
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
	ClientInstalled bool
	DaemonRunning   bool
	Connected       bool
	PeerID          string
	PeerName        string
	WG0IP           string
}

func (s *NetBirdService) Status(ctx context.Context) (NetBirdStatus, error) {
	if s == nil || s.projects == nil {
		return NetBirdStatus{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	currentMode, currentModeKnown := configuredNetBirdMode(s.cfg.NetBirdMode)
	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdStatus{}, errs.Wrap(errs.CodeNetBirdStatusFailed, "failed to load netbird status project catalog", err)
	}
	catalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      currentMode,
		PanelPort: panelPort,
		Projects:  projectInputs,
	})

	status := NetBirdStatus{
		CurrentMode:          currentMode,
		LastPolicySyncStatus: netBirdSyncStatusNever,
		APIReachability: NetBirdAPIReachability{
			Source: netBirdReachabilitySourceCatalog,
		},
		ManagedGroups:      len(catalog.Groups),
		ManagedPolicies:    len(catalog.Policies),
		ManagedCountSource: netBirdManagedCountSourceCatalog,
		Warnings:           []string{},
	}

	if !currentModeKnown {
		status.Warnings = append(status.Warnings, "Current mode is not configured; status assumed legacy mode.")
	}
	if panelPortFallback {
		status.Warnings = append(status.Warnings, "Panel port was not a valid integer; status used default port 8080.")
	}
	status.Warnings = append(status.Warnings, projectWarnings...)

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

	apiToken := ""
	apiBaseURL := ""
	hostPeerID := ""
	if snapshot.RequestParsed {
		apiToken = strings.TrimSpace(snapshot.Request.APIToken)
		apiBaseURL = strings.TrimSpace(snapshot.Request.APIBaseURL)
		hostPeerID = strings.TrimSpace(snapshot.Request.HostPeerID)
	}
	if apiToken == "" {
		status.APIReachability.Source = netBirdReachabilitySourceLastSync
		status.APIReachable = status.LastPolicySyncStatus == netBirdSyncStatusSucceeded
		if !status.APIReachable && status.APIReachability.Message == "" {
			status.APIReachability.Message = "No API token was available from the latest mode apply job."
		}
		return status, nil
	}

	liveCheckedAt := time.Now().UTC()
	status.APIReachability.Source = netBirdReachabilitySourceLive
	status.APIReachability.CheckedAt = &liveCheckedAt
	live, err := fetchNetBirdLiveStatus(ctx, apiBaseURL, apiToken, hostPeerID)
	if err != nil {
		status.APIReachability.Message = fmt.Sprintf("Live NetBird API check failed: %v", err)
		return status, nil
	}

	status.APIReachable = true
	status.ManagedGroups = live.ManagedGroups
	status.ManagedPolicies = live.ManagedPolicies
	status.ManagedCountSource = netBirdManagedCountSourceAPI
	status.ClientInstalled = live.ClientInstalled
	status.DaemonRunning = live.DaemonRunning
	status.Connected = live.Connected
	status.PeerID = live.PeerID
	status.PeerName = live.PeerName
	status.WG0IP = live.WG0IP

	return status, nil
}

func (s *NetBirdService) ACLGraph(ctx context.Context) (NetBirdACLGraph, error) {
	if s == nil || s.projects == nil {
		return NetBirdACLGraph{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	currentMode, currentModeKnown := configuredNetBirdMode(s.cfg.NetBirdMode)
	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdACLGraph{}, errs.Wrap(errs.CodeNetBirdACLGraphFailed, "failed to load netbird acl graph project catalog", err)
	}
	catalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      currentMode,
		PanelPort: panelPort,
		Projects:  projectInputs,
	})

	graph := buildNetBirdACLGraph(currentMode, catalog, projectInputs)
	if !currentModeKnown {
		graph.Notes = append(graph.Notes, "Current mode is not configured; ACL graph assumed legacy mode.")
	}
	if panelPortFallback {
		graph.Notes = append(graph.Notes, "Panel port was not a valid integer; ACL graph used default port 8080.")
	}
	graph.Notes = append(graph.Notes, projectWarnings...)
	if currentMode == NetBirdModeLegacy {
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

	result.Found = true
	result.Job = *job

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
		return result, nil
	}
	result.Summary = summary
	return result, nil
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
	groups, err := client.ListGroups(ctx)
	if err != nil {
		return netBirdLiveStatus{}, err
	}
	policies, err := client.ListPolicies(ctx)
	if err != nil {
		return netBirdLiveStatus{}, err
	}

	result := netBirdLiveStatus{
		ManagedGroups:   countManagedGroups(groups),
		ManagedPolicies: countManagedPolicies(policies),
	}

	host := resolveHostPeer(peers, groups, hostPeerID)
	if host == nil {
		return result, nil
	}

	result.ClientInstalled = true
	result.DaemonRunning = host.Connected
	result.Connected = host.Connected
	result.PeerID = strings.TrimSpace(host.ID)
	result.PeerName = strings.TrimSpace(host.Name)
	result.WG0IP = strings.TrimSpace(host.IP)
	return result, nil
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
