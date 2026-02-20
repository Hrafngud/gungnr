package service

const (
	JobTypeCreateTemplate = "create_template"
	JobTypeDeployExisting = "deploy_existing"
	JobTypeQuickService   = "quick_service"
	JobTypeForwardLocal   = "forward_local"
	JobTypeProjectArchive = "project_archive"
	JobTypeDockerRun      = "docker_run"
	JobTypeDockerCompose  = "docker_compose_up"
	JobTypeHostRestart    = "host_restart_project_stack"
)
