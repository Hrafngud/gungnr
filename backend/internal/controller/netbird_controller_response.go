package controller

import (
	"net/http"
	"strings"

	"go-notes/internal/errs"
)

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
