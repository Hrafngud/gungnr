package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

var projectComposeFileCandidates = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

type projectPathResolution struct {
	RequestedName  string
	NormalizedName string
	ProjectDir     string
	Source         string
	ComposeFiles   []string
	EnvPath        string
	EnvExists      bool
	ProjectRecord  *models.Project
}

func resolveProjectPath(
	ctx context.Context,
	repo repository.ProjectRepository,
	templatesDir string,
	projectName string,
) (projectPathResolution, error) {
	requested := strings.TrimSpace(projectName)
	normalized := strings.ToLower(requested)
	if err := ValidateProjectName(normalized); err != nil {
		return projectPathResolution{}, errs.New(errs.CodeProjectInvalidName, "project name must be lowercase alphanumerics or dashes")
	}

	record, err := lookupProjectRecord(ctx, repo, normalized)
	if err != nil {
		return projectPathResolution{}, err
	}

	resolution := projectPathResolution{
		RequestedName:  requested,
		NormalizedName: normalized,
		ProjectRecord:  record,
	}

	if dir, ok := resolveDirFromRecord(record, templatesDir); ok {
		resolution.ProjectDir = dir
		resolution.Source = "db_path"
	} else {
		dir, err := resolveDirFromTemplatesScan(templatesDir, normalized)
		if err != nil {
			return projectPathResolution{}, err
		}
		resolution.ProjectDir = dir
		resolution.Source = "templates_scan"
	}

	resolution.ComposeFiles = existingComposeFiles(resolution.ProjectDir)
	resolution.EnvPath, resolution.EnvExists = resolveProjectEnvPath(resolution.ProjectDir)

	return resolution, nil
}

func lookupProjectRecord(
	ctx context.Context,
	repo repository.ProjectRepository,
	normalizedName string,
) (*models.Project, error) {
	if repo == nil {
		return nil, nil
	}

	findByName := func(name string) (*models.Project, error) {
		project, err := repo.GetByName(ctx, name)
		if err == nil {
			return project, nil
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if project, err := findByName(normalizedName); err != nil {
		return nil, err
	} else if project != nil {
		return project, nil
	}

	projects, err := repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range projects {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if name == normalizedName {
			project := item
			return &project, nil
		}
	}
	return nil, nil
}

func resolveDirFromRecord(record *models.Project, templatesDir string) (string, bool) {
	if record == nil {
		return "", false
	}

	rawPath := strings.TrimSpace(record.Path)
	if rawPath == "" {
		return "", false
	}

	candidates := make([]string, 0, 4)
	if filepath.IsAbs(rawPath) {
		candidates = append(candidates, rawPath)
		if templatesDir != "" {
			candidates = append(candidates, filepath.Join(templatesDir, filepath.Base(rawPath)))
		}
	} else {
		candidates = append(candidates, rawPath)
		if templatesDir != "" {
			candidates = append(candidates, filepath.Join(templatesDir, rawPath))
		}
	}
	if templatesDir != "" {
		candidates = append(candidates, filepath.Join(templatesDir, filepath.Base(rawPath)))
	}

	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		info, err := os.Stat(cleaned)
		if err != nil || !info.IsDir() {
			continue
		}
		return cleaned, true
	}

	return "", false
}

func resolveDirFromTemplatesScan(templatesDir, normalizedName string) (string, error) {
	baseDir := strings.TrimSpace(templatesDir)
	if baseDir == "" {
		return "", errs.New(errs.CodeProjectNotFound, "project not found")
	}

	exactDir := filepath.Join(baseDir, normalizedName)
	if info, err := os.Stat(exactDir); err == nil && info.IsDir() {
		return exactDir, nil
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", errs.Wrap(errs.CodeProjectDetailFailed, "failed to read templates directory", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), normalizedName) {
			return filepath.Join(baseDir, entry.Name()), nil
		}
	}

	return "", errs.New(errs.CodeProjectNotFound, fmt.Sprintf("project %q not found", normalizedName))
}

func existingComposeFiles(projectDir string) []string {
	files := make([]string, 0, len(projectComposeFileCandidates))
	for _, name := range projectComposeFileCandidates {
		candidate := filepath.Join(projectDir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			files = append(files, candidate)
		}
	}
	sort.Strings(files)
	return files
}

func resolveProjectEnvPath(projectDir string) (string, bool) {
	defaultPath := filepath.Join(projectDir, ".env")
	if info, err := os.Stat(defaultPath); err == nil && !info.IsDir() {
		return defaultPath, true
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return defaultPath, false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), ".env") {
			candidate := filepath.Join(projectDir, entry.Name())
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}

	return defaultPath, false
}
