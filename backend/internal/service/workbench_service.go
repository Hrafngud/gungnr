package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
	"go-notes/internal/repository"
	"go-notes/internal/validate"
)

const defaultWorkbenchLockWaitTimeout = 15 * time.Second
const (
	defaultWorkbenchBackupMaxCount = 10
	defaultWorkbenchBackupMaxAge   = 30 * 24 * time.Hour
)

type WorkbenchComposeSource struct {
	ProjectName string `json:"projectName"`
	ProjectDir  string `json:"projectDir"`
	ComposePath string `json:"composePath"`
	Fingerprint string `json:"fingerprint"`
	Normalized  string `json:"normalized"`
	Raw         []byte `json:"-"`
}

type WorkbenchService struct {
	templatesDir      string
	projects          repository.ProjectRepository
	settings          repository.SettingsRepository
	sessionSecret     string
	hostPortScanner   workbenchHostPortScanner
	runtimeMetaClient infraDockerMetadataClient
	hostReadClient    infraProjectFileReadClient
	fileClient        infraProjectFileMutationClient
	lockManager       *workbenchProjectLockManager
	lockWaitTimeout   time.Duration
	backupMaxCount    int
	backupMaxAge      time.Duration
	nowFn             func() time.Time
}

func NewWorkbenchService(templatesDir string, projects repository.ProjectRepository) *WorkbenchService {
	return NewWorkbenchServiceWithStorage(templatesDir, projects, nil, "")
}

func NewWorkbenchServiceWithStorage(
	templatesDir string,
	projects repository.ProjectRepository,
	settings repository.SettingsRepository,
	sessionSecret string,
) *WorkbenchService {
	return &WorkbenchService{
		templatesDir:      strings.TrimSpace(templatesDir),
		projects:          projects,
		settings:          settings,
		sessionSecret:     strings.TrimSpace(sessionSecret),
		hostPortScanner:   workbenchScanOccupiedHostPortsWithProbeClient(nil),
		runtimeMetaClient: nil,
		hostReadClient:    nil,
		fileClient:        nil,
		lockManager:       newWorkbenchProjectLockManager(),
		lockWaitTimeout:   defaultWorkbenchLockWaitTimeout,
		backupMaxCount:    defaultWorkbenchBackupMaxCount,
		backupMaxAge:      defaultWorkbenchBackupMaxAge,
		nowFn:             time.Now,
	}
}

func (s *WorkbenchService) SetPortProbeClient(probeClient infraPortProbeClient) {
	s.hostPortScanner = workbenchScanOccupiedHostPortsWithProbeClient(probeClient)
}

func (s *WorkbenchService) SetRuntimeMetaClient(runtimeMetaClient infraDockerMetadataClient) {
	s.runtimeMetaClient = runtimeMetaClient
}

func (s *WorkbenchService) SetHostFileReadClient(fileClient infraProjectFileReadClient) {
	s.hostReadClient = fileClient
}

func (s *WorkbenchService) SetFileMutationClient(fileClient infraProjectFileMutationClient) {
	s.fileClient = fileClient
}

func (s *WorkbenchService) AcquireProjectLock(ctx context.Context, projectName string) (func(), error) {
	normalized, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return nil, err
	}
	return s.lockManager.Acquire(ctx, normalized, s.lockWaitTimeout)
}

func (s *WorkbenchService) ResolveComposeSource(ctx context.Context, projectName string) (WorkbenchComposeSource, error) {
	resolved, err := resolveProjectPath(ctx, s.projects, s.templatesDir, projectName, s.runtimeMetaClient)
	if err != nil {
		return WorkbenchComposeSource{}, err
	}

	composePath, err := resolveWorkbenchComposePath(resolved)
	if err != nil {
		return WorkbenchComposeSource{}, err
	}

	raw, err := readProjectFile(ctx, s.hostReadClient, resolved.ProjectDir, composePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WorkbenchComposeSource{}, workbenchSourceNotFoundError(resolved)
		}
		return WorkbenchComposeSource{}, workbenchSourceInvalidError(resolved, composePath, "failed to read compose source", err)
	}

	normalized, fingerprint := WorkbenchSourceFingerprint(raw)
	return WorkbenchComposeSource{
		ProjectName: resolved.NormalizedName,
		ProjectDir:  resolved.ProjectDir,
		ComposePath: composePath,
		Fingerprint: fingerprint,
		Normalized:  normalized,
		Raw:         raw,
	}, nil
}

func (s *WorkbenchService) ResolveComposeSourceWithLock(
	ctx context.Context,
	projectName string,
) (WorkbenchComposeSource, func(), error) {
	release, err := s.AcquireProjectLock(ctx, projectName)
	if err != nil {
		return WorkbenchComposeSource{}, nil, err
	}

	source, err := s.ResolveComposeSource(ctx, projectName)
	if err != nil {
		release()
		return WorkbenchComposeSource{}, nil, err
	}
	return source, release, nil
}

func (s *WorkbenchService) ParseComposeCore(ctx context.Context, projectName string) (WorkbenchComposeParseResult, error) {
	source, release, err := s.ResolveComposeSourceWithLock(ctx, projectName)
	if err != nil {
		return WorkbenchComposeParseResult{}, err
	}
	defer release()

	return s.ParseComposeCoreFromSource(source)
}

func (s *WorkbenchService) ParseComposeCoreFromSource(source WorkbenchComposeSource) (WorkbenchComposeParseResult, error) {
	result, err := ParseWorkbenchComposeCore(source.Normalized)
	if err != nil {
		return WorkbenchComposeParseResult{}, errs.WithDetails(
			errs.Wrap(errs.CodeWorkbenchSourceInvalid, "failed to parse compose source", err),
			map[string]any{
				"project":     source.ProjectName,
				"projectPath": source.ProjectDir,
				"composePath": source.ComposePath,
				"fingerprint": source.Fingerprint,
			},
		)
	}

	result.ProjectName = source.ProjectName
	result.ProjectDir = source.ProjectDir
	result.ComposePath = source.ComposePath
	result.SourceFingerprint = source.Fingerprint
	return result, nil
}

func normalizeWorkbenchProjectName(projectName string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(projectName))
	if err := validate.ProjectName(normalized); err != nil {
		return "", errs.New(errs.CodeProjectInvalidName, "project name must be lowercase alphanumerics or dashes")
	}
	return normalized, nil
}

func resolveWorkbenchComposePath(resolution projectPathResolution) (string, error) {
	for _, name := range projectComposeFileCandidates {
		candidate := filepath.Join(resolution.ProjectDir, name)
		path, ok, err := sanitizeWorkbenchComposePath(resolution.ProjectDir, candidate)
		if err != nil {
			return "", workbenchSourceInvalidError(resolution, candidate, "invalid compose source path", err)
		}
		if ok {
			return path, nil
		}
	}

	for _, candidate := range resolution.ComposeFiles {
		path, ok, err := sanitizeWorkbenchComposePath(resolution.ProjectDir, candidate)
		if err != nil {
			return "", workbenchSourceInvalidError(resolution, candidate, "invalid compose source path", err)
		}
		if ok {
			return path, nil
		}
	}

	for _, candidate := range resolution.ComposeFiles {
		path, err := normalizeWorkbenchProjectPath(resolution.ProjectDir, candidate)
		if err != nil {
			return "", workbenchSourceInvalidError(resolution, candidate, "invalid compose source path", err)
		}
		if path != "" {
			return path, nil
		}
	}

	return "", workbenchSourceNotFoundError(resolution)
}

func sanitizeWorkbenchComposePath(projectDir, composePath string) (string, bool, error) {
	cleaned, err := normalizeWorkbenchProjectPath(projectDir, composePath)
	if err != nil || cleaned == "" {
		return cleaned, false, err
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("compose source path points to a directory")
	}

	return cleaned, true, nil
}

func normalizeWorkbenchProjectPath(projectDir, filePath string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(filePath))
	if cleaned == "" {
		return "", nil
	}

	projectRoot := filepath.Clean(strings.TrimSpace(projectDir))
	if projectRoot == "" {
		return "", fmt.Errorf("project root is empty")
	}
	if rootResolved, err := filepath.EvalSymlinks(projectRoot); err == nil {
		projectRoot = rootResolved
	}

	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(projectRoot, cleaned)
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = resolved
	}
	if !isPathWithinBase(projectRoot, cleaned) {
		return "", fmt.Errorf("compose source path resolves outside project root")
	}
	return cleaned, nil
}

func readProjectFile(
	ctx context.Context,
	fileClient infraProjectFileReadClient,
	projectDir string,
	filePath string,
) ([]byte, error) {
	raw, err := os.ReadFile(filePath)
	if err == nil || fileClient == nil {
		return raw, err
	}

	result, bridgeErr := fileClient.ProjectFileRead(ctx, "", contract.ProjectFileReadPayload{
		BasePath: projectDir,
		Path:     filePath,
	})
	if bridgeErr != nil {
		return nil, err
	}
	if result.Status != contract.StatusSucceeded {
		return nil, err
	}
	content, ok := result.Data["content"].(string)
	if !ok {
		return nil, fmt.Errorf("infra bridge project_file_read returned invalid content payload")
	}
	return []byte(content), nil
}

func workbenchSourceNotFoundError(resolution projectPathResolution) error {
	message := fmt.Sprintf("compose source not found for project %q", resolution.NormalizedName)
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchSourceNotFound, message),
		map[string]any{
			"project":     resolution.NormalizedName,
			"projectPath": resolution.ProjectDir,
			"candidates":  append([]string(nil), projectComposeFileCandidates...),
		},
	)
}

func workbenchSourceInvalidError(
	resolution projectPathResolution,
	composePath string,
	message string,
	cause error,
) error {
	details := map[string]any{
		"project":     resolution.NormalizedName,
		"projectPath": resolution.ProjectDir,
		"composePath": composePath,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchSourceInvalid, message, cause), details)
}

type workbenchProjectLockManager struct {
	locks sync.Map
}

type workbenchProjectLock struct {
	token chan struct{}
}

func newWorkbenchProjectLockManager() *workbenchProjectLockManager {
	return &workbenchProjectLockManager{}
}

func (m *workbenchProjectLockManager) Acquire(
	ctx context.Context,
	projectName string,
	timeout time.Duration,
) (func(), error) {
	lock := m.projectLock(projectName)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			return nil, errs.WithDetails(
				errs.New(errs.CodeWorkbenchLocked, fmt.Sprintf("workbench lock is held for project %q", projectName)),
				map[string]any{
					"project":            projectName,
					"waitTimeoutSeconds": int(timeout.Seconds()),
				},
			)
		}
		return nil, waitCtx.Err()
	case <-lock.token:
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			lock.token <- struct{}{}
		})
	}, nil
}

func (m *workbenchProjectLockManager) projectLock(projectName string) *workbenchProjectLock {
	existing, ok := m.locks.Load(projectName)
	if ok {
		return existing.(*workbenchProjectLock)
	}

	lock := &workbenchProjectLock{
		token: make(chan struct{}, 1),
	}
	lock.token <- struct{}{}

	actual, _ := m.locks.LoadOrStore(projectName, lock)
	return actual.(*workbenchProjectLock)
}
