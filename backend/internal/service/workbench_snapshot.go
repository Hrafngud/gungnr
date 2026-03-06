package service

import (
	"context"
	"path/filepath"
)

func (s *WorkbenchService) GetSnapshot(
	ctx context.Context,
	projectName string,
) (WorkbenchStackSnapshot, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}

	resolution, err := resolveProjectPath(ctx, s.projects, s.templatesDir, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}
	if exists {
		return snapshot, nil
	}

	return WorkbenchStackSnapshot{
		ProjectName:       resolution.NormalizedName,
		ProjectDir:        resolution.ProjectDir,
		ComposePath:       workbenchSnapshotComposePath(resolution),
		ModelVersion:      workbenchModelVersion,
		Revision:          0,
		SourceFingerprint: "",
		Services:          []WorkbenchComposeService{},
		Dependencies:      []WorkbenchComposeDependency{},
		Ports:             []WorkbenchComposePort{},
		Resources:         []WorkbenchComposeResource{},
		NetworkRefs:       []WorkbenchComposeNetworkRef{},
		VolumeRefs:        []WorkbenchComposeVolumeRef{},
		EnvRefs:           []WorkbenchComposeEnvRef{},
		Modules:           []WorkbenchStackModule{},
		Warnings:          []WorkbenchComposeWarning{},
	}, nil
}

func workbenchSnapshotComposePath(resolution projectPathResolution) string {
	if composePath, err := resolveWorkbenchComposePath(resolution); err == nil {
		return composePath
	}
	if resolution.ProjectDir == "" {
		return ""
	}
	return filepath.Join(resolution.ProjectDir, projectComposeFileCandidates[0])
}
