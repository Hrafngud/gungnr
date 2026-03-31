package models

import "time"

// --- Requests ---

// ProjectContainerActionRequest is the request body for project container stop/restart.
type ProjectContainerActionRequest struct {
	Container string `json:"container"`
}

// ProjectRemoveContainerActionRequest is the request body for project container removal.
type ProjectRemoveContainerActionRequest struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"removeVolumes"`
}

// ProjectEnvWriteRequest is the request body for writing a project .env file.
type ProjectEnvWriteRequest struct {
	Content      string `json:"content"`
	CreateBackup *bool  `json:"createBackup,omitempty"`
}

// ProjectArchiveRequest is the request body for archiving a project.
type ProjectArchiveRequest struct {
	RemoveContainers *bool `json:"removeContainers,omitempty"`
	RemoveVolumes    *bool `json:"removeVolumes,omitempty"`
	RemoveIngress    *bool `json:"removeIngress,omitempty"`
	RemoveDNS        *bool `json:"removeDns,omitempty"`
}

// ProjectWorkbenchImportRequest is the request body for importing a workbench snapshot.
type ProjectWorkbenchImportRequest struct {
	Reason string `json:"reason,omitempty"`
}

// WorkbenchPortSelector identifies a port in a workbench snapshot.
type WorkbenchPortSelector struct {
	ServiceName   string `json:"serviceName"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"hostIp,omitempty"`
}

// ProjectWorkbenchPortMutationRequest is the request body for mutating a workbench port.
type ProjectWorkbenchPortMutationRequest struct {
	Selector       WorkbenchPortSelector `json:"selector"`
	Action         string                `json:"action"`
	ManualHostPort *int                  `json:"manualHostPort,omitempty"`
}

// ProjectWorkbenchPortSuggestionRequest is the request body for suggesting workbench ports.
type ProjectWorkbenchPortSuggestionRequest struct {
	Selector WorkbenchPortSelector `json:"selector"`
	Limit    int                   `json:"limit,omitempty"`
}

// WorkbenchModuleSelector identifies a module in a workbench snapshot.
type WorkbenchModuleSelector struct {
	ServiceName string `json:"serviceName"`
	ModuleType  string `json:"moduleType"`
}

// ProjectWorkbenchResourceMutationRequest is the request body for mutating service resources.
type ProjectWorkbenchResourceMutationRequest struct {
	Action            string   `json:"action"`
	LimitCPUs         *string  `json:"limitCpus,omitempty"`
	LimitMemory       *string  `json:"limitMemory,omitempty"`
	ReservationCPUs   *string  `json:"reservationCpus,omitempty"`
	ReservationMemory *string  `json:"reservationMemory,omitempty"`
	ClearFields       []string `json:"clearFields,omitempty"`
}

// ProjectWorkbenchOptionalServiceAddRequest is the request body for adding an optional service.
type ProjectWorkbenchOptionalServiceAddRequest struct {
	EntryKey string `json:"entryKey"`
}

// ProjectWorkbenchModuleMutationRequest is the request body for mutating workbench modules.
type ProjectWorkbenchModuleMutationRequest struct {
	Selector WorkbenchModuleSelector `json:"selector"`
	Action   string                  `json:"action"`
}

// ProjectWorkbenchComposePreviewRequest is the request body for previewing compose changes.
type ProjectWorkbenchComposePreviewRequest struct {
	ExpectedRevision *int `json:"expectedRevision,omitempty"`
}

// ProjectWorkbenchComposeApplyRequest is the request body for applying compose changes.
type ProjectWorkbenchComposeApplyRequest struct {
	ExpectedRevision          *int   `json:"expectedRevision,omitempty"`
	ExpectedSourceFingerprint string `json:"expectedSourceFingerprint,omitempty"`
}

// ProjectWorkbenchComposeRestoreRequest is the request body for restoring a compose backup.
type ProjectWorkbenchComposeRestoreRequest struct {
	BackupID string `json:"backupId"`
}

// --- Responses ---

// ProjectResponse is the API response shape for a project.
type ProjectResponse struct {
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

// NewProjectResponse builds a ProjectResponse from a Project model.
func NewProjectResponse(project Project) ProjectResponse {
	return ProjectResponse{
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

// NewProjectResponses builds a slice of ProjectResponse from Project models.
func NewProjectResponses(projects []Project) []ProjectResponse {
	response := make([]ProjectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, NewProjectResponse(project))
	}
	return response
}
