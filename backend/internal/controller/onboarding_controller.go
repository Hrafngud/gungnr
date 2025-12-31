package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type OnboardingController struct {
	service *service.OnboardingService
	audit   *service.AuditService
}

func NewOnboardingController(service *service.OnboardingService, audit *service.AuditService) *OnboardingController {
	return &OnboardingController{service: service, audit: audit}
}

func (c *OnboardingController) Register(r gin.IRoutes) {
	r.GET("/onboarding", c.Get)
	r.PATCH("/onboarding", c.Update)
}

func (c *OnboardingController) Get(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	state, err := c.service.Get(ctx.Request.Context(), session.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load onboarding state"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"state": state})
}

func (c *OnboardingController) Update(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	var req service.OnboardingUpdatePayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	state, err := c.service.Update(ctx.Request.Context(), session.UserID, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update onboarding state"})
		return
	}

	c.logAudit(ctx, req)

	ctx.JSON(http.StatusOK, gin.H{"state": state})
}

func (c *OnboardingController) logAudit(ctx *gin.Context, req service.OnboardingUpdatePayload) {
	if c.audit == nil {
		return
	}
	session, _ := middleware.SessionFromContext(ctx)
	metadata := map[string]any{}
	if req.Home != nil {
		metadata["home"] = *req.Home
	}
	if req.HostSettings != nil {
		metadata["hostSettings"] = *req.HostSettings
	}
	if req.Networking != nil {
		metadata["networking"] = *req.Networking
	}
	if req.GitHub != nil {
		metadata["github"] = *req.GitHub
	}
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    session.UserID,
		UserLogin: session.Login,
		Action:    "onboarding.update",
		Target:    "onboarding",
		Metadata:  metadata,
	})
}
