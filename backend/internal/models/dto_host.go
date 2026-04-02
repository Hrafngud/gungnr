package models

// ContainerActionRequest is the request body for container stop/restart.
type ContainerActionRequest struct {
	Container string `json:"container"`
}

// RemoveContainerRequest is the request body for container removal.
type RemoveContainerRequest struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"removeVolumes"`
}

// ProjectActionRequest is the request body for project-level host actions.
type ProjectActionRequest struct {
	Project string `json:"project"`
}
