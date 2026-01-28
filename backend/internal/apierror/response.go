package apierror

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
	"go-notes/internal/errs"
	cf "go-notes/internal/integrations/cloudflare"
	gh "go-notes/internal/integrations/github"
	"go-notes/internal/service"
)

type Response struct {
	Code    errs.Code         `json:"code"`
	Message string            `json:"message"`
	Error   string            `json:"error"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details any               `json:"details,omitempty"`
	DocsURL string            `json:"docsUrl,omitempty"`
}

type Options struct {
	Fields  map[string]string
	Details any
}

func Respond(ctx *gin.Context, status int, code errs.Code, message string, opts *Options) {
	payload := Response{
		Code:    code,
		Message: message,
		Error:   message,
		DocsURL: docsURL(code),
	}
	if opts != nil {
		payload.Fields = opts.Fields
		payload.Details = opts.Details
	}
	ctx.JSON(status, payload)
}

func RespondWithError(ctx *gin.Context, status int, err error, fallbackCode errs.Code, fallbackMessage string) {
	code, message, options := classify(err, fallbackCode, fallbackMessage)
	Respond(ctx, status, code, message, options)
}

func classify(err error, fallbackCode errs.Code, fallbackMessage string) (errs.Code, string, *Options) {
	if err == nil {
		return fallbackCode, fallbackMessage, nil
	}

	if typed, ok := errs.From(err); ok {
		message := strings.TrimSpace(typed.Message)
		if message == "" {
			message = fallbackMessage
		}
		return typed.Code, message, &Options{Fields: typed.Fields, Details: typed.Details}
	}

	if code, message, ok := mapKnown(err); ok {
		return code, message, nil
	}

	return fallbackCode, fallbackMessage, nil
}

func mapKnown(err error) (errs.Code, string, bool) {
	switch {
	case errors.Is(err, auth.ErrInvalidSession), errors.Is(err, auth.ErrExpiredSession):
		return errs.CodeAuthUnauthenticated, "unauthenticated", true
	case errors.Is(err, service.ErrUnauthorized):
		return errs.CodeAuthForbidden, "user not allowed", true
	case errors.Is(err, service.ErrAdminAuthDisabled):
		return errs.CodeAuthTestTokenDisabled, "test token disabled", true
	case errors.Is(err, service.ErrJobAlreadyFinished):
		return errs.CodeJobAlreadyFinished, service.ErrJobAlreadyFinished.Error(), true
	case errors.Is(err, service.ErrJobRunning):
		return errs.CodeJobRunning, service.ErrJobRunning.Error(), true
	case errors.Is(err, service.ErrJobNotStoppable):
		return errs.CodeJobNotStoppable, service.ErrJobNotStoppable.Error(), true
	case errors.Is(err, service.ErrJobNotRetryable):
		return errs.CodeJobNotRetryable, service.ErrJobNotRetryable.Error(), true
	case errors.Is(err, service.ErrLastSuperUser):
		return errs.CodeUserLastSuperUser, service.ErrLastSuperUser.Error(), true
	case errors.Is(err, service.ErrAllowlistUserNotFound):
		return errs.CodeUserGitHubNotFound, service.ErrAllowlistUserNotFound.Error(), true
	case errors.Is(err, service.ErrAllowlistLoginRequired):
		return errs.CodeUserLoginRequired, service.ErrAllowlistLoginRequired.Error(), true
	case errors.Is(err, service.ErrCannotRemoveSuperUser):
		return errs.CodeUserRemoveSuperUser, service.ErrCannotRemoveSuperUser.Error(), true
	case errors.Is(err, cf.ErrMissingToken):
		return errs.CodeCloudflareMissingToken, cf.ErrMissingToken.Error(), true
	case errors.Is(err, cf.ErrMissingAccountID):
		return errs.CodeCloudflareMissingAccount, cf.ErrMissingAccountID.Error(), true
	case errors.Is(err, cf.ErrMissingZoneID):
		return errs.CodeCloudflareMissingZone, cf.ErrMissingZoneID.Error(), true
	case errors.Is(err, cf.ErrMissingTunnel):
		return errs.CodeCloudflareMissingTunnel, cf.ErrMissingTunnel.Error(), true
	case errors.Is(err, cf.ErrTunnelNotRemote):
		return errs.CodeCloudflareTunnelLocal, cf.ErrTunnelNotRemote.Error(), true
	case errors.Is(err, gh.ErrMissingToken):
		return errs.CodeGitHubMissingToken, gh.ErrMissingToken.Error(), true
	default:
		return "", "", false
	}
}

func docsURL(code errs.Code) string {
	trimmed := strings.TrimSpace(string(code))
	if trimmed == "" {
		return ""
	}
	return "/errors.html#" + trimmed
}
