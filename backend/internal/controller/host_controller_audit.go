package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

func (c *HostController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if _, ok := metadata["container"]; !ok {
		if _, hasProject := metadata["project"]; !hasProject {
			metadata["container"] = target
		}
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
