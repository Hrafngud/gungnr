package controller

import (
	"time"

	"go-notes/internal/models"
	"go-notes/internal/service"
)

type projectResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	RepoURL   string    `json:"repoUrl"`
	Path      string    `json:"path"`
	ProxyPort int       `json:"proxyPort"`
	DBPort    int       `json:"dbPort"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newProjectResponseFromModel(project models.Project) projectResponse {
	return projectResponse{
		ID:        project.ID,
		Name:      project.Name,
		RepoURL:   project.RepoURL,
		Path:      project.Path,
		ProxyPort: project.ProxyPort,
		DBPort:    project.DBPort,
		Status:    project.Status,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}
}

func newProjectResponsesFromModels(projects []models.Project) []projectResponse {
	response := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, newProjectResponseFromModel(project))
	}
	return response
}

func newProjectResponseFromSummary(project service.ProjectSummary) projectResponse {
	return projectResponse{
		ID:        project.ID,
		Name:      project.Name,
		RepoURL:   project.RepoURL,
		Path:      project.Path,
		ProxyPort: project.ProxyPort,
		DBPort:    project.DBPort,
		Status:    project.Status,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}
}

func newProjectResponsesFromSummaries(projects []service.ProjectSummary) []projectResponse {
	response := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, newProjectResponseFromSummary(project))
	}
	return response
}
