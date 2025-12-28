package main

import (
	"log"

	"go-notes/internal/auth"
	"go-notes/internal/config"
	"go-notes/internal/controller"
	"go-notes/internal/db"
	"go-notes/internal/integrations/cloudflare"
	gh "go-notes/internal/integrations/github"
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

	authService := service.NewAuthService(cfg, userRepo)
	jobRunner := jobs.NewRunner(jobRepo)
	jobService := service.NewJobService(jobRepo, jobRunner)
	projectService := service.NewProjectService(cfg, projectRepo, jobService)

	githubClient := gh.NewClient(cfg)
	cloudflareClient := cloudflare.NewClient(cfg)
	workflows := service.NewProjectWorkflows(cfg, projectRepo, cloudflareClient, githubClient)
	workflows.Register(jobRunner)

	sessionManager := auth.NewManager(cfg.SessionSecret, cfg.SessionTTL)
	secureCookie := cfg.AppEnv == "prod"
	cookieDomain := cfg.CookieDomain

	r := router.NewRouter(router.Dependencies{
		Health:         controller.NewHealthController(),
		Auth:           controller.NewAuthController(authService, sessionManager, secureCookie, cookieDomain),
		Projects:       controller.NewProjectsController(projectService),
		Jobs:           controller.NewJobsController(jobService),
		AllowedOrigins: cfg.AllowedOrigins,
		AuthMiddleware: middleware.AuthRequired(sessionManager),
	})

	log.Printf("server starting on %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
