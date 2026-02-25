package main

import (
	"context"
	"log"
	"time"

	"go-notes/internal/auth"
	"go-notes/internal/config"
	"go-notes/internal/controller"
	"go-notes/internal/db"
	infraclient "go-notes/internal/infra/client"
	"go-notes/internal/infra/contract"
	infraqueue "go-notes/internal/infra/queue"
	infraworker "go-notes/internal/infra/worker"
	"go-notes/internal/jobs"
	"go-notes/internal/middleware"
	"go-notes/internal/repository"
	"go-notes/internal/router"
	"go-notes/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(gormDB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	if err := db.CleanupLegacyHostWorker(gormDB); err != nil {
		log.Printf("warn: legacy host-worker cleanup failed: %v", err)
	}

	userRepo := repository.NewGormUserRepository(gormDB)
	projectRepo := repository.NewGormProjectRepository(gormDB)
	jobRepo := repository.NewGormJobRepository(gormDB)
	settingsRepo := repository.NewGormSettingsRepository(gormDB)
	auditRepo := repository.NewGormAuditLogRepository(gormDB)

	rbacService := service.NewRBACService(cfg, userRepo)
	if err := rbacService.SeedSuperUser(); err != nil {
		log.Fatalf("failed to seed superuser: %v", err)
	}

	authService := service.NewAuthService(cfg, userRepo)
	jobRunner := jobs.NewRunner(jobRepo)
	jobService := service.NewJobService(jobRepo, jobRunner)
	settingsService := service.NewSettingsService(cfg, settingsRepo)
	userService := service.NewUserService(userRepo)
	githubService := service.NewGitHubService(cfg, settingsService)
	cloudflareService := service.NewCloudflareService(settingsService)
	netBirdService := service.NewNetBirdService(cfg, projectRepo, jobRepo)
	auditService := service.NewAuditService(auditRepo)
	dockerRunner := service.NewDockerRunner()

	bridgeQueue, err := infraqueue.NewFilesystem(cfg.InfraQueueRoot)
	if err != nil {
		log.Fatalf("failed to initialize infra queue: %v", err)
	}
	cleanupReport, cleanupErr := bridgeQueue.CleanupStale(context.Background(), time.Now().UTC(), infraqueue.RetentionPolicy{
		IntentMaxAge: cfg.InfraIntentMaxAge,
		ResultMaxAge: cfg.InfraResultMaxAge,
		ClaimMaxAge:  cfg.InfraClaimMaxAge,
	})
	if cleanupErr != nil {
		log.Printf("warn: infra queue cleanup failed: %v", cleanupErr)
	} else if cleanupReport.TotalRemoved() > 0 {
		log.Printf(
			"infra queue cleanup removed intents=%d results=%d claims=%d protected=%d",
			cleanupReport.RemovedIntents,
			cleanupReport.RemovedResults,
			cleanupReport.RemovedClaims,
			cleanupReport.ProtectedTasks,
		)
	}
	bridgeClient := infraclient.New(bridgeQueue, cfg.InfraPollInterval, cfg.InfraResultTimeout)
	bridgeWorker := infraworker.New(bridgeQueue, cfg.InfraPollInterval, cfg.TemplatesDir, log.Default())
	if err := bridgeWorker.ValidateTaskCoverage([]contract.TaskType{
		contract.TaskTypeRestartTunnel,
		contract.TaskTypeDockerStopContainer,
		contract.TaskTypeDockerRestartContainer,
		contract.TaskTypeDockerRemoveContainer,
		contract.TaskTypeComposeUpStack,
	}); err != nil {
		log.Fatalf("infra worker readiness check failed: %v", err)
	}
	go bridgeWorker.Run(context.Background())

	hostService := service.NewHostService(cfg.TemplatesDir, projectRepo, bridgeClient)
	projectService := service.NewProjectService(cfg, projectRepo, jobService, settingsService)
	projectArchiveService := service.NewProjectArchiveService(cfg, projectRepo, settingsService, jobService, hostService)
	projectRuntimeService := service.NewProjectRuntimeService(cfg.TemplatesDir, projectRepo, hostService)
	projectEnvService := service.NewProjectEnvService(cfg.TemplatesDir, projectRepo)
	healthService := service.NewHealthService(hostService, settingsService)

	workflows := service.NewProjectWorkflows(cfg, projectRepo, settingsService, hostService, auditService, dockerRunner, bridgeClient)
	workflows.Register(jobRunner)
	dockerWorkflows := service.NewDockerWorkflows(dockerRunner)
	dockerWorkflows.Register(jobRunner)
	hostWorkflows := service.NewHostWorkflows(hostService)
	hostWorkflows.Register(jobRunner)
	netBirdWorkflows := service.NewNetBirdWorkflows(netBirdService, auditService)
	netBirdWorkflows.Register(jobRunner)

	sessionManager := auth.NewManager(cfg.SessionSecret, cfg.SessionTTL)
	secureCookie := cfg.AppEnv == "prod"
	cookieDomain := cfg.CookieDomain

	r := router.NewRouter(router.Dependencies{
		Health:          controller.NewHealthController(healthService),
		Auth:            controller.NewAuthController(authService, auditService, sessionManager, secureCookie, cookieDomain),
		Projects:        controller.NewProjectsController(projectService, projectArchiveService, projectRuntimeService, projectEnvService, hostService, jobService, auditService),
		Jobs:            controller.NewJobsController(jobService),
		Settings:        controller.NewSettingsController(settingsService, auditService),
		Host:            controller.NewHostController(hostService, jobService, auditService),
		NetBird:         controller.NewNetBirdController(netBirdService, jobService, auditService),
		Audit:           controller.NewAuditController(auditService),
		Users:           controller.NewUsersController(userService),
		GitHub:          controller.NewGitHubController(githubService),
		Cloudflare:      controller.NewCloudflareController(cloudflareService),
		AllowedOrigins:  cfg.AllowedOrigins,
		AuthMiddleware:  middleware.AuthRequired(sessionManager),
		UsersMiddleware: middleware.RequireAdmin(sessionManager),
	})

	log.Printf("server starting on %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
