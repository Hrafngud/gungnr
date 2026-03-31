package controller

import (
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
)

type SettingsController struct {
	service *service.SettingsService
	audit   *service.AuditService
}

func NewSettingsController(service *service.SettingsService, audit *service.AuditService) *SettingsController {
	return &SettingsController{service: service, audit: audit}
}

func (c *SettingsController) Get(ctx *gin.Context) {
	settings, err := c.service.Get(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsLoadFailed, "failed to load settings")
		return
	}
	response, err := c.buildResponse(ctx, settings)
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}
	respond.OK(ctx, response)
}

func (c *SettingsController) Update(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeSettingsAdminRequired, "admin role required"), errs.CodeSettingsAdminRequired, "admin role required")
		return
	}

	var req service.SettingsPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeSettingsInvalidBody, "invalid request body"), errs.CodeSettingsInvalidBody, "invalid request body")
		return
	}

	settings, err := c.service.Update(ctx.Request.Context(), req)
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsUpdateFailed, "failed to update settings")
		return
	}

	c.logAudit(ctx, "settings.update", "settings", map[string]any{
		"baseDomain":               req.BaseDomain,
		"additionalDomainsCount":   len(req.AdditionalDomains),
		"githubTemplatesCount":     templateCount(req.GitHubTemplates),
		"githubAppId":              req.GitHubAppID,
		"githubAppClientId":        req.GitHubAppClientID,
		"githubAppClientSecretSet": req.GitHubAppClientSecret != "",
		"githubAppInstallationId":  req.GitHubAppInstallationID,
		"githubAppPrivateKeySet":   req.GitHubAppPrivateKey != "",
		"cloudflareTokenSet":       req.CloudflareToken != "",
		"cloudflareAccountId":      req.CloudflareAccountID,
		"cloudflareZoneId":         req.CloudflareZoneID,
		"cloudflaredTunnel":        req.CloudflaredTunnel,
		"cloudflaredConfigPath":    req.CloudflaredConfigPath,
	})

	response, err := c.buildResponse(ctx, settings)
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}

	respond.OK(ctx, response)
}

func (c *SettingsController) CloudflaredPreview(ctx *gin.Context) {
	preview, err := c.service.CloudflaredPreview(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsPreviewFailed, err.Error())
		return
	}

	respond.OK(ctx, gin.H{"preview": preview})
}

func (c *SettingsController) SyncCloudflareFromEnv(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeSettingsAdminRequired, "admin role required"), errs.CodeSettingsAdminRequired, "admin role required")
		return
	}

	settings, err := c.service.SyncCloudflareFromEnv(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsSyncFailed, "failed to sync Cloudflare settings")
		return
	}

	c.logAudit(ctx, "settings.cloudflare.sync", "settings", map[string]any{
		"cloudflareTokenSet":    settings.CloudflareToken != "",
		"cloudflareAccountId":   settings.CloudflareAccountID,
		"cloudflareZoneId":      settings.CloudflareZoneID,
		"cloudflaredTunnel":     settings.CloudflaredTunnel,
		"cloudflaredConfigPath": settings.CloudflaredConfigPath,
	})

	response, err := c.buildResponse(ctx, settings)
	if err != nil {
		respond.Err(ctx, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}

	respond.OK(ctx, response)
}

func (c *SettingsController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
	}
	session, _ := middleware.SessionFromContext(ctx)
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    session.UserID,
		UserLogin: session.Login,
		Action:    action,
		Target:    target,
		Metadata:  metadata,
	})
}

func (c *SettingsController) buildResponse(ctx *gin.Context, settings service.SettingsPayload) (models.SettingsFullResponse, error) {
	cfg, sources, err := c.service.ResolveConfigWithSources(ctx.Request.Context())
	if err != nil {
		return models.SettingsFullResponse{}, err
	}
	return models.SettingsFullResponse{
		Settings:              settings,
		Sources:               sources,
		CloudflaredTunnelName: strings.TrimSpace(cfg.CloudflaredTunnel),
		TemplatesDir:          strings.TrimSpace(cfg.TemplatesDir),
	}, nil
}

func templateCount(templates []service.GitHubTemplateSource) any {
	if templates == nil {
		return "unchanged"
	}
	return len(templates)
}

func isAdminRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case models.RoleAdmin, models.RoleSuperUser:
		return true
	default:
		return false
	}
}
