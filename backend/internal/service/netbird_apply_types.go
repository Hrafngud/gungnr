package service

import (
	"strings"
	"time"
)

type NetBirdModeApplyRequest struct {
	TargetMode     string   `json:"targetMode"`
	AllowLocalhost bool     `json:"allowLocalhost"`
	APIBaseURL     string   `json:"apiBaseUrl,omitempty"`
	APIToken       string   `json:"apiToken"`
	HostPeerID     string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs   []string `json:"adminPeerIds,omitempty"`
}

type NetBirdModeApplyActor struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

type NetBirdModeApplyJobRequest struct {
	TargetMode     string                `json:"targetMode"`
	AllowLocalhost bool                  `json:"allowLocalhost"`
	APIBaseURL     string                `json:"apiBaseUrl,omitempty"`
	APIToken       string                `json:"apiToken"`
	HostPeerID     string                `json:"hostPeerId,omitempty"`
	AdminPeerIDs   []string              `json:"adminPeerIds,omitempty"`
	RequestedBy    NetBirdModeApplyActor `json:"requestedBy"`
	RequestedAt    time.Time             `json:"requestedAt"`
}

type NetBirdOperationCounts struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

type NetBirdModeApplySummary struct {
	CurrentMode         NetBirdMode            `json:"currentMode"`
	TargetMode          NetBirdMode            `json:"targetMode"`
	AllowLocalhost      bool                   `json:"allowLocalhost"`
	DefaultPolicyAction string                 `json:"defaultPolicyAction"`
	Plan                NetBirdModePlan        `json:"plan"`
	Reconcile           NetBirdReconcileResult `json:"reconcile"`
	GroupResultCounts   NetBirdOperationCounts `json:"groupResultCounts"`
	PolicyResultCounts  NetBirdOperationCounts `json:"policyResultCounts"`
	Warnings            []string               `json:"warnings"`
	RequestedBy         NetBirdModeApplyActor  `json:"requestedBy"`
	RequestedAt         time.Time              `json:"requestedAt"`
	CompletedAt         time.Time              `json:"completedAt"`
}

func NormalizeNetBirdModeApplyRequest(input NetBirdModeApplyRequest) NetBirdModeApplyRequest {
	return NetBirdModeApplyRequest{
		TargetMode:     strings.ToLower(strings.TrimSpace(input.TargetMode)),
		AllowLocalhost: input.AllowLocalhost,
		APIBaseURL:     strings.TrimSpace(input.APIBaseURL),
		APIToken:       strings.TrimSpace(input.APIToken),
		HostPeerID:     strings.TrimSpace(input.HostPeerID),
		AdminPeerIDs:   normalizeStringList(input.AdminPeerIDs),
	}
}

func BuildNetBirdModeApplyJobRequest(input NetBirdModeApplyRequest, actor NetBirdModeApplyActor) NetBirdModeApplyJobRequest {
	normalized := NormalizeNetBirdModeApplyRequest(input)
	return NetBirdModeApplyJobRequest{
		TargetMode:     normalized.TargetMode,
		AllowLocalhost: normalized.AllowLocalhost,
		APIBaseURL:     normalized.APIBaseURL,
		APIToken:       normalized.APIToken,
		HostPeerID:     normalized.HostPeerID,
		AdminPeerIDs:   normalized.AdminPeerIDs,
		RequestedBy: NetBirdModeApplyActor{
			UserID: actor.UserID,
			Login:  strings.TrimSpace(actor.Login),
		},
		RequestedAt: time.Now().UTC(),
	}
}
