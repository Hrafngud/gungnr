package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
	"go-notes/internal/repository"
)

const defaultProjectEnvMaxBytes int64 = 1 << 20 // 1 MiB

type ProjectEnvRead struct {
	Path      string     `json:"path"`
	Exists    bool       `json:"exists"`
	SizeBytes int64      `json:"sizeBytes"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	Content   string     `json:"content"`
}

type ProjectEnvWrite struct {
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"sizeBytes"`
	UpdatedAt  time.Time `json:"updatedAt"`
	BackupPath string    `json:"backupPath,omitempty"`
}

type ProjectEnvService struct {
	templatesDir      string
	projects          repository.ProjectRepository
	maxBytes          int64
	runtimeMetaClient infraDockerMetadataClient
	fileClient        infraProjectFileMutationClient
}

func NewProjectEnvService(templatesDir string, projects repository.ProjectRepository) *ProjectEnvService {
	return &ProjectEnvService{
		templatesDir:      strings.TrimSpace(templatesDir),
		projects:          projects,
		maxBytes:          defaultProjectEnvMaxBytes,
		runtimeMetaClient: nil,
		fileClient:        nil,
	}
}

func (s *ProjectEnvService) SetRuntimeMetaClient(runtimeMetaClient infraDockerMetadataClient) {
	s.runtimeMetaClient = runtimeMetaClient
}

func (s *ProjectEnvService) SetFileMutationClient(fileClient infraProjectFileMutationClient) {
	s.fileClient = fileClient
}

func (s *ProjectEnvService) Load(ctx context.Context, projectName string) (ProjectEnvRead, error) {
	resolved, err := resolveProjectPath(ctx, s.projects, s.templatesDir, projectName, s.runtimeMetaClient)
	if err != nil {
		return ProjectEnvRead{}, err
	}

	response := ProjectEnvRead{
		Path:    resolved.EnvPath,
		Exists:  false,
		Content: "",
	}

	exists, sizeBytes, updatedAt := envFileInfo(resolved.EnvPath)
	response.Exists = exists
	response.SizeBytes = sizeBytes
	response.UpdatedAt = updatedAt

	if !exists {
		return response, nil
	}
	if sizeBytes > s.maxBytes {
		return ProjectEnvRead{}, errs.New(
			errs.CodeProjectEnvTooLarge,
			fmt.Sprintf(".env exceeds max size (%d bytes)", s.maxBytes),
		)
	}

	content, err := os.ReadFile(resolved.EnvPath)
	if err != nil {
		return ProjectEnvRead{}, errs.Wrap(errs.CodeProjectEnvReadFailed, "failed to read .env", err)
	}

	response.Content = string(content)
	return response, nil
}

func (s *ProjectEnvService) Save(
	ctx context.Context,
	projectName string,
	content string,
	createBackup bool,
) (ProjectEnvWrite, error) {
	if int64(len(content)) > s.maxBytes {
		return ProjectEnvWrite{}, errs.New(
			errs.CodeProjectEnvTooLarge,
			fmt.Sprintf(".env exceeds max size (%d bytes)", s.maxBytes),
		)
	}

	resolved, err := resolveProjectPath(ctx, s.projects, s.templatesDir, projectName, s.runtimeMetaClient)
	if err != nil {
		return ProjectEnvWrite{}, err
	}

	projectDir := resolved.ProjectDir
	envPath := resolved.EnvPath

	if !isPathWithinBase(projectDir, envPath) {
		return ProjectEnvWrite{}, errs.New(errs.CodeProjectEnvWriteFailed, "unsafe .env path")
	}

	if existing, err := os.Lstat(envPath); err == nil && existing.Mode()&os.ModeSymlink != 0 {
		return ProjectEnvWrite{}, errs.New(errs.CodeProjectEnvWriteFailed, "refusing to write through symlinked .env")
	}
	if s.fileClient == nil {
		return ProjectEnvWrite{}, errs.New(errs.CodeProjectEnvWriteFailed, "infra bridge file client unavailable")
	}

	backupPath := ""
	if createBackup {
		if exists, _, _ := envFileInfo(envPath); exists {
			backupPath = filepath.Join(projectDir, fmt.Sprintf(".env.backup.%s", time.Now().UTC().Format("20060102-150405")))
			if !isPathWithinBase(projectDir, backupPath) {
				return ProjectEnvWrite{}, errs.New(errs.CodeProjectEnvWriteFailed, "unsafe .env backup path")
			}
			if _, err := s.fileClient.ProjectFileCopy(ctx, "", contract.ProjectFileCopyPayload{
				BasePath:        projectDir,
				SourcePath:      envPath,
				DestinationPath: backupPath,
				Mode:            0o600,
				CreateParents:   true,
			}); err != nil {
				return ProjectEnvWrite{}, errs.Wrap(errs.CodeProjectEnvWriteFailed, "failed to create .env backup", err)
			}
		}
	}

	if _, err := s.fileClient.ProjectFileWriteAtomic(ctx, "", contract.ProjectFileWriteAtomicPayload{
		BasePath:      projectDir,
		Path:          envPath,
		Content:       content,
		Mode:          0o600,
		PreserveMode:  false,
		CreateParents: true,
	}); err != nil {
		return ProjectEnvWrite{}, errs.Wrap(errs.CodeProjectEnvWriteFailed, "failed to replace .env file", err)
	}

	info, err := os.Stat(envPath)
	if err != nil {
		return ProjectEnvWrite{}, errs.Wrap(errs.CodeProjectEnvWriteFailed, "failed to stat saved .env file", err)
	}
	updatedAt := info.ModTime().UTC()
	return ProjectEnvWrite{
		Path:       envPath,
		SizeBytes:  info.Size(),
		UpdatedAt:  updatedAt,
		BackupPath: backupPath,
	}, nil
}

func isPathWithinBase(baseDir, target string) bool {
	base := filepath.Clean(baseDir)
	targetPath := filepath.Clean(target)
	rel, err := filepath.Rel(base, targetPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}
