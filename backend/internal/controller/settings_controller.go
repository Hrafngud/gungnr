package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/service"
)

type SettingsResponse struct {
	Settings              service.SettingsPayload `json:"settings"`
	Sources               service.SettingsSources `json:"sources,omitempty"`
	CloudflaredTunnelName string                  `json:"cloudflaredTunnelName,omitempty"`
	TemplatesDir          string                  `json:"templatesDir,omitempty"`
}

type SettingsController struct {
	service *service.SettingsService
	audit   *service.AuditService
}

func NewSettingsController(service *service.SettingsService, audit *service.AuditService) *SettingsController {
	return &SettingsController{service: service, audit: audit}
}

func (c *SettingsController) Register(r gin.IRoutes) {
	r.GET("/settings", c.Get)
	r.PUT("/settings", c.Update)
	r.GET("/settings/cloudflared/preview", c.CloudflaredPreview)
	r.POST("/settings/cloudflare/sync", c.SyncCloudflareFromEnv)
}

func (c *SettingsController) Get(ctx *gin.Context) {
	settings, err := c.service.Get(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsLoadFailed, "failed to load settings")
		return
	}
	response, err := c.buildResponse(ctx, settings)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func (c *SettingsController) Update(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeSettingsAdminRequired, "admin role required", nil)
		return
	}

	var req service.SettingsPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeSettingsInvalidBody, "invalid request body", nil)
		return
	}

	settings, err := c.service.Update(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsUpdateFailed, "failed to update settings")
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
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *SettingsController) CloudflaredPreview(ctx *gin.Context) {
	preview, err := c.service.CloudflaredPreview(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeSettingsPreviewFailed, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"preview": preview})
}

func (c *SettingsController) SyncCloudflareFromEnv(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeSettingsAdminRequired, "admin role required", nil)
		return
	}

	settings, err := c.service.SyncCloudflareFromEnv(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsSyncFailed, "failed to sync Cloudflare settings")
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
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeSettingsSourcesFailed, "failed to load settings sources")
		return
	}

	ctx.JSON(http.StatusOK, response)
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

func (c *SettingsController) buildResponse(ctx *gin.Context, settings service.SettingsPayload) (SettingsResponse, error) {
	cfg, sources, err := c.service.ResolveConfigWithSources(ctx.Request.Context())
	if err != nil {
		return SettingsResponse{}, err
	}
	return SettingsResponse{
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
