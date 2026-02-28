package service

import (
	"sort"
	"strings"
	"time"
)

type NetBirdModeApplyRequest struct {
	TargetMode      string   `json:"targetMode"`
	AllowLocalhost  bool     `json:"allowLocalhost"`
	ModeBProjectIDs []uint   `json:"modeBProjectIds,omitempty"`
	APIBaseURL      string   `json:"apiBaseUrl,omitempty"`
	APIToken        string   `json:"apiToken"`
	HostPeerID      string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    []string `json:"adminPeerIds,omitempty"`
}

type NetBirdModeApplyActor struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

type NetBirdPolicyReapplyRequest struct {
	APIBaseURL   string   `json:"apiBaseUrl,omitempty"`
	APIToken     string   `json:"apiToken,omitempty"`
	HostPeerID   string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs []string `json:"adminPeerIds,omitempty"`
}

type NetBirdModeApplyJobRequest struct {
	TargetMode      string                `json:"targetMode"`
	AllowLocalhost  bool                  `json:"allowLocalhost"`
	ModeBProjectIDs []uint                `json:"modeBProjectIds,omitempty"`
	APIBaseURL      string                `json:"apiBaseUrl,omitempty"`
	APIToken        string                `json:"apiToken"`
	HostPeerID      string                `json:"hostPeerId,omitempty"`
	AdminPeerIDs    []string              `json:"adminPeerIds,omitempty"`
	RequestedBy     NetBirdModeApplyActor `json:"requestedBy"`
	RequestedAt     time.Time             `json:"requestedAt"`
}

type NetBirdOperationCounts struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

type NetBirdExecutionCounts struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type NetBirdExecutionDiagnostics struct {
	IntentID        string `json:"intentId,omitempty"`
	WorkerErrorCode string `json:"workerErrorCode,omitempty"`
	LogPath         string `json:"logPath,omitempty"`
}

type NetBirdRebindingExecutionOperation struct {
	Service       string                       `json:"service"`
	ProjectID     uint                         `json:"projectId,omitempty"`
	ProjectName   string                       `json:"projectName,omitempty"`
	Port          int                          `json:"port"`
	FromListeners []string                     `json:"fromListeners"`
	ToListeners   []string                     `json:"toListeners"`
	Reason        string                       `json:"reason,omitempty"`
	Result        string                       `json:"result"`
	Message       string                       `json:"message,omitempty"`
	RequestID     string                       `json:"requestId,omitempty"`
	Diagnostics   *NetBirdExecutionDiagnostics `json:"diagnostics,omitempty"`
}

type NetBirdRebindingExecutionSummary struct {
	Counts     NetBirdExecutionCounts               `json:"counts"`
	Operations []NetBirdRebindingExecutionOperation `json:"operations"`
}

type NetBirdRedeployExecutionTarget struct {
	Service     string                       `json:"service"`
	ProjectID   uint                         `json:"projectId,omitempty"`
	ProjectName string                       `json:"projectName,omitempty"`
	Port        int                          `json:"port,omitempty"`
	Reason      string                       `json:"reason,omitempty"`
	Result      string                       `json:"result"`
	Message     string                       `json:"message,omitempty"`
	RequestID   string                       `json:"requestId,omitempty"`
	Diagnostics *NetBirdExecutionDiagnostics `json:"diagnostics,omitempty"`
}

type NetBirdRedeployExecutionSummary struct {
	Counts   NetBirdExecutionCounts           `json:"counts"`
	Panel    *NetBirdRedeployExecutionTarget  `json:"panel,omitempty"`
	Projects []NetBirdRedeployExecutionTarget `json:"projects"`
}

type NetBirdModeApplySummary struct {
	CurrentMode           NetBirdMode                      `json:"currentMode"`
	TargetMode            NetBirdMode                      `json:"targetMode"`
	AllowLocalhost        bool                             `json:"allowLocalhost"`
	TargetModeBProjectIDs []uint                           `json:"targetModeBProjectIds,omitempty"`
	DefaultPolicyAction   string                           `json:"defaultPolicyAction"`
	Plan                  NetBirdModePlan                  `json:"plan"`
	Reconcile             NetBirdReconcileResult           `json:"reconcile"`
	RebindingExecution    NetBirdRebindingExecutionSummary `json:"rebindingExecution"`
	RedeployExecution     NetBirdRedeployExecutionSummary  `json:"redeployExecution"`
	GroupResultCounts     NetBirdOperationCounts           `json:"groupResultCounts"`
	PolicyResultCounts    NetBirdOperationCounts           `json:"policyResultCounts"`
	Warnings              []string                         `json:"warnings"`
	RequestedBy           NetBirdModeApplyActor            `json:"requestedBy"`
	RequestedAt           time.Time                        `json:"requestedAt"`
	CompletedAt           time.Time                        `json:"completedAt"`
}

type NetBirdDefaultPolicySummary struct {
	Action string                    `json:"action"`
	Result NetBirdReconcileOperation `json:"result"`
}

type NetBirdPolicyReapplySummary struct {
	CurrentMode        NetBirdMode                 `json:"currentMode"`
	DefaultPolicy      NetBirdDefaultPolicySummary `json:"defaultPolicy"`
	GroupResultCounts  NetBirdOperationCounts      `json:"groupResultCounts"`
	PolicyResultCounts NetBirdOperationCounts      `json:"policyResultCounts"`
	GroupOperations    []NetBirdReconcileOperation `json:"groupOperations"`
	PolicyOperations   []NetBirdReconcileOperation `json:"policyOperations"`
	Warnings           []string                    `json:"warnings"`
}

func NormalizeNetBirdModeApplyRequest(input NetBirdModeApplyRequest) NetBirdModeApplyRequest {
	return NetBirdModeApplyRequest{
		TargetMode:      strings.ToLower(strings.TrimSpace(input.TargetMode)),
		AllowLocalhost:  input.AllowLocalhost,
		ModeBProjectIDs: normalizeUintList(input.ModeBProjectIDs),
		APIBaseURL:      strings.TrimSpace(input.APIBaseURL),
		APIToken:        strings.TrimSpace(input.APIToken),
		HostPeerID:      strings.TrimSpace(input.HostPeerID),
		AdminPeerIDs:    normalizeStringList(input.AdminPeerIDs),
	}
}

func NormalizeNetBirdPolicyReapplyRequest(input NetBirdPolicyReapplyRequest) NetBirdPolicyReapplyRequest {
	return NetBirdPolicyReapplyRequest{
		APIBaseURL:   strings.TrimSpace(input.APIBaseURL),
		APIToken:     strings.TrimSpace(input.APIToken),
		HostPeerID:   strings.TrimSpace(input.HostPeerID),
		AdminPeerIDs: normalizeStringList(input.AdminPeerIDs),
	}
}

func BuildNetBirdModeApplyJobRequest(input NetBirdModeApplyRequest, actor NetBirdModeApplyActor) NetBirdModeApplyJobRequest {
	normalized := NormalizeNetBirdModeApplyRequest(input)
	return NetBirdModeApplyJobRequest{
		TargetMode:      normalized.TargetMode,
		AllowLocalhost:  normalized.AllowLocalhost,
		ModeBProjectIDs: normalized.ModeBProjectIDs,
		APIBaseURL:      normalized.APIBaseURL,
		// Do not persist API tokens in async job payloads.
		APIToken:     "",
		HostPeerID:   normalized.HostPeerID,
		AdminPeerIDs: normalized.AdminPeerIDs,
		RequestedBy: NetBirdModeApplyActor{
			UserID: actor.UserID,
			Login:  strings.TrimSpace(actor.Login),
		},
		RequestedAt: time.Now().UTC(),
	}
}

func normalizeUintList(values []uint) []uint {
	set := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		set[value] = struct{}{}
	}
	out := make([]uint, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
