package main

import (
	"log"

	"go-notes/internal/config"
	"go-notes/internal/controller"
	"go-notes/internal/db"
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

	noteRepo := repository.NewGormNoteRepository(gormDB)
	noteService := service.NewNoteService(noteRepo)

	r := router.NewRouter(router.Dependencies{
		Health:         controller.NewHealthController(),
		Notes:          controller.NewNoteController(noteService),
		AllowedOrigins: cfg.AllowedOrigins,
	})

	log.Printf("server starting on %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
