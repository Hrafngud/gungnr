package service

import (
	"context"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"gorm.io/gorm"
)

func TestPlanMode_ModeBUsesSelectedProjectsOnly(t *testing.T) {
	svc := &NetBirdService{
		cfg: config.Config{
			Port:                  "8080",
			NetBirdMode:           string(NetBirdModeB),
			NetBirdAllowLocalhost: false,
		},
		projects: fakeNetBirdServiceProjectRepo{
			projects: []models.Project{
				{Model: gorm.Model{ID: 1}, Name: "alpha", ProxyPort: 18080},
				{Model: gorm.Model{ID: 2}, Name: "beta", ProxyPort: 28080},
			},
		},
	}

	plan, err := svc.PlanMode(context.Background(), string(NetBirdModeB), false, []uint{2})
	if err != nil {
		t.Fatalf("PlanMode returned error: %v", err)
	}

	if len(plan.TargetModeBProjectIDs) != 1 || plan.TargetModeBProjectIDs[0] != 2 {
		t.Fatalf("expected target mode b project ids [2], got %v", plan.TargetModeBProjectIDs)
	}

	hasAlpha := false
	hasBeta := false
	for _, policy := range plan.Catalog.Policies {
		if policy.Name == netBirdModeBProjectPolicyName(1) {
			hasAlpha = true
		}
		if policy.Name == netBirdModeBProjectPolicyName(2) {
			hasBeta = true
		}
	}
	if hasAlpha {
		t.Fatal("expected alpha project policy to be excluded from mode b catalog")
	}
	if !hasBeta {
		t.Fatal("expected beta project policy to be included in mode b catalog")
	}

	foundProjectRebind := false
	for _, op := range plan.ServiceRebindingOperations {
		if op.ProjectID != 2 {
			continue
		}
		foundProjectRebind = true
		if len(op.FromListeners) != 1 || op.FromListeners[0] != "0.0.0.0" {
			t.Fatalf("expected from listeners [0.0.0.0], got %v", op.FromListeners)
		}
		if len(op.ToListeners) != 1 || op.ToListeners[0] != "wg0" {
			t.Fatalf("expected to listeners [wg0], got %v", op.ToListeners)
		}
	}
	if !foundProjectRebind {
		t.Fatal("expected a project rebinding operation for selected project id 2")
	}
}

type fakeNetBirdServiceProjectRepo struct {
	projects []models.Project
}

func (f fakeNetBirdServiceProjectRepo) List(context.Context) ([]models.Project, error) {
	return append([]models.Project(nil), f.projects...), nil
}

func (fakeNetBirdServiceProjectRepo) Create(context.Context, *models.Project) error { return nil }

func (fakeNetBirdServiceProjectRepo) GetByName(context.Context, string) (*models.Project, error) {
	return nil, repository.ErrNotFound
}

func (fakeNetBirdServiceProjectRepo) Update(context.Context, *models.Project) error { return nil }
