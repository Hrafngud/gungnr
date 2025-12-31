package main

import (
	"log"
	"time"

	"go-notes/internal/auth"
	"go-notes/internal/config"
	"go-notes/internal/controller"
	"go-notes/internal/db"
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

	userRepo := repository.NewGormUserRepository(gormDB)
	projectRepo := repository.NewGormProjectRepository(gormDB)
	jobRepo := repository.NewGormJobRepository(gormDB)
	settingsRepo := repository.NewGormSettingsRepository(gormDB)
	auditRepo := repository.NewGormAuditLogRepository(gormDB)
	onboardingRepo := repository.NewGormOnboardingRepository(gormDB)

	authService := service.NewAuthService(cfg, userRepo)
	jobRunner := jobs.NewRunner(jobRepo)
	jobService := service.NewJobService(jobRepo, jobRunner)
	hostJobService := service.NewHostJobService(jobRepo, 30*time.Minute)
	projectService := service.NewProjectService(cfg, projectRepo, jobService)
	settingsService := service.NewSettingsService(cfg, settingsRepo)
	onboardingService := service.NewOnboardingService(onboardingRepo)
	githubService := service.NewGitHubService(cfg, settingsService)
	cloudflareService := service.NewCloudflareService(settingsService)
	auditService := service.NewAuditService(auditRepo)
	hostService := service.NewHostService()
	healthService := service.NewHealthService(hostService, settingsService)

	workflows := service.NewProjectWorkflows(cfg, projectRepo, settingsService)
	workflows.Register(jobRunner)

	sessionManager := auth.NewManager(cfg.SessionSecret, cfg.SessionTTL)
	secureCookie := cfg.AppEnv == "prod"
	cookieDomain := cfg.CookieDomain

	r := router.NewRouter(router.Dependencies{
		Health:         controller.NewHealthController(healthService),
		Auth:           controller.NewAuthController(authService, auditService, sessionManager, secureCookie, cookieDomain),
		Projects:       controller.NewProjectsController(projectService, auditService),
		Jobs:           controller.NewJobsController(jobService),
		HostJobs:       controller.NewHostJobsController(hostJobService, auditService),
		Settings:       controller.NewSettingsController(settingsService, auditService),
		Onboarding:     controller.NewOnboardingController(onboardingService, auditService),
		Host:           controller.NewHostController(hostService),
		Audit:          controller.NewAuditController(auditService),
		GitHub:         controller.NewGitHubController(githubService),
		Cloudflare:     controller.NewCloudflareController(cloudflareService),
		AllowedOrigins: cfg.AllowedOrigins,
		AuthMiddleware: middleware.AuthRequired(sessionManager),
	})

	log.Printf("server starting on %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
