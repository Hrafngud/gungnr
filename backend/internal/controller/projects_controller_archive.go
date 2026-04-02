package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
)

func (c *ProjectsController) ArchivePlan(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeProjectAdminRequired, "admin role required"), errs.CodeProjectAdminRequired, "admin role required")
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.archive == nil {
		respond.Err(ctx, errs.New(errs.CodeProjectArchivePlanFailed, "project archive service unavailable"), errs.CodeProjectArchivePlanFailed, "project archive service unavailable")
		return
	}

	plan, err := c.archive.Plan(ctx.Request.Context(), project)
	if err != nil {
		respond.Err(ctx, err, errs.CodeProjectArchivePlanFailed, "failed to build project archive plan")
		return
	}

	c.logAudit(ctx, "project.archive.plan", plan.Project.NormalizedName, map[string]any{
		"project":        plan.Project.NormalizedName,
		"containers":     len(plan.Containers),
		"hostnames":      len(plan.Hostnames),
		"ingressRules":   len(plan.Ingress),
		"dnsRecords":     len(plan.DNSRecords),
		"warningCount":   len(plan.Warnings),
		"defaultOptions": plan.Defaults,
	})

	respond.OK(ctx, gin.H{"plan": plan})
}

func (c *ProjectsController) Archive(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeProjectAdminRequired, "admin role required"), errs.CodeProjectAdminRequired, "admin role required")
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.archive == nil {
		respond.Err(ctx, errs.New(errs.CodeProjectArchiveFailed, "project archive service unavailable"), errs.CodeProjectArchiveFailed, "project archive service unavailable")
		return
	}

	options, ok := c.parseProjectArchiveRequest(ctx)
	if !ok {
		return
	}

	job, plan, err := c.archive.Queue(ctx.Request.Context(), project, options, service.ProjectArchiveActor{
		UserID: session.UserID,
		Login:  session.Login,
	})
	if err != nil {
		respond.Err(ctx, err, errs.CodeProjectArchiveFailed, "failed to queue project archive")
		return
	}

	targets := jobTargetsFromPlan(plan, options)
	c.logAudit(ctx, "project.archive.execute", plan.Project.NormalizedName, map[string]any{
		"project":          plan.Project.NormalizedName,
		"jobId":            job.ID,
		"removeContainers": options.RemoveContainers,
		"removeVolumes":    options.RemoveVolumes,
		"removeIngress":    options.RemoveIngress,
		"removeDns":        options.RemoveDNS,
		"targets": map[string]any{
			"containers": len(targets.Containers),
			"hostnames":  len(targets.Hostnames),
			"dnsRecords": len(targets.DNSRecords),
		},
		"warningCount": len(plan.Warnings),
	})

	respond.Accepted(ctx, gin.H{
		"job":  models.NewJobResponse(*job),
		"plan": plan,
	})
}
