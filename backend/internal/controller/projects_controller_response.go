package controller

import (
	"go-notes/internal/models"
	"go-notes/internal/service"
)

func newProjectResponseFromSummary(project service.ProjectSummary) models.ProjectResponse {
	return models.ProjectResponse{
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

func newProjectResponsesFromSummaries(projects []service.ProjectSummary) []models.ProjectResponse {
	response := make([]models.ProjectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, newProjectResponseFromSummary(project))
	}
	return response
}
