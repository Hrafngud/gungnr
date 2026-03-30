package controller

import (
	"net/http"
	"strings"

	"go-notes/internal/errs"
)

type netBirdModePlanRequest struct {
	TargetMode      string `json:"targetMode"`
	AllowLocalhost  bool   `json:"allowLocalhost"`
	ModeBProjectIDs []uint `json:"modeBProjectIds,omitempty"`
}

type netBirdModeConfigUpsertRequest struct {
	APIBaseURL      *string   `json:"apiBaseUrl,omitempty"`
	APIToken        *string   `json:"apiToken,omitempty"`
	HostPeerID      *string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    *[]string `json:"adminPeerIds,omitempty"`
	ModeBProjectIDs *[]uint   `json:"modeBProjectIds,omitempty"`
}

func netBirdHTTPStatus(err error, fallback int) int {
	typed, ok := errs.From(err)
	if !ok {
		return fallback
	}
	switch typed.Code {
	case errs.CodeNetBirdInvalidMode, errs.CodeNetBirdInvalidBody:
		return http.StatusBadRequest
	case errs.CodeNetBirdUnavailable:
		return http.StatusInternalServerError
	case errs.CodeNetBirdStatusFailed, errs.CodeNetBirdACLGraphFailed, errs.CodeNetBirdPlanFailed:
		return http.StatusBadGateway
	case errs.CodeNetBirdApplyFailed:
		return http.StatusInternalServerError
	case errs.CodeNetBirdReapplyFailed:
		return http.StatusInternalServerError
	default:
		if strings.HasPrefix(string(typed.Code), "NETBIRD-400") {
			return http.StatusBadRequest
		}
		return fallback
	}
}
