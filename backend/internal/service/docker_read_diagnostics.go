package service

import (
	"errors"
	"strings"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
)

type DockerReadDiagnostic struct {
	Scope      string `json:"scope"`
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	SourceCode string `json:"sourceCode,omitempty"`
	TaskType   string `json:"taskType,omitempty"`
}

const (
	dockerReadDiagnosticStatusDegraded = "degraded"

	dockerReadDiagnosticScopeRuntime             = "runtime"
	dockerReadDiagnosticScopeContainers          = "containers"
	dockerReadDiagnosticScopeUsageSummary        = "usage.summary"
	dockerReadDiagnosticScopeUsageProjectCounts  = "usage.projectCounts"
	dockerReadDiagnosticCodeRuntimeDegraded      = "DOCKER-RUNTIME-DEGRADED"
	dockerReadDiagnosticCodeContainersDegraded   = "DOCKER-CONTAINERS-DEGRADED"
	dockerReadDiagnosticCodeUsageDegraded        = "DOCKER-USAGE-DEGRADED"
	dockerReadDiagnosticCodeProjectCountsLimited = "DOCKER-PROJECT-COUNTS-DEGRADED"
)

type dockerUsageProjectCountsDegradedError struct {
	cause error
}

func (e dockerUsageProjectCountsDegradedError) Error() string {
	if e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e dockerUsageProjectCountsDegradedError) Unwrap() error {
	return e.cause
}

func degradedDockerRuntimeDiagnostics(err error) []DockerReadDiagnostic {
	return []DockerReadDiagnostic{
		buildDockerReadDiagnostic(
			dockerReadDiagnosticScopeRuntime,
			dockerReadDiagnosticCodeRuntimeDegraded,
			"Docker runtime details are unavailable; configured Docker posture is shown without live daemon confirmation.",
			err,
		),
	}
}

func degradedDockerContainerDiagnostics(err error) []DockerReadDiagnostic {
	return []DockerReadDiagnostic{
		buildDockerReadDiagnostic(
			dockerReadDiagnosticScopeContainers,
			dockerReadDiagnosticCodeContainersDegraded,
			"Docker-backed container inventory is unavailable; the response is showing an empty inventory shell.",
			err,
		),
	}
}

func degradedDockerUsageDiagnostics(project string, err error) []DockerReadDiagnostic {
	if isDockerUsageProjectCountsDegraded(err) {
		project = strings.TrimSpace(project)
		if project == "" {
			return nil
		}
		return []DockerReadDiagnostic{
			buildDockerReadDiagnostic(
				dockerReadDiagnosticScopeUsageProjectCounts,
				dockerReadDiagnosticCodeProjectCountsLimited,
				"Project-scoped Docker usage counts are unavailable because Docker-backed inventory reads failed.",
				err,
			),
		}
	}

	diagnostics := []DockerReadDiagnostic{
		buildDockerReadDiagnostic(
			dockerReadDiagnosticScopeUsageSummary,
			dockerReadDiagnosticCodeUsageDegraded,
			"Docker-backed usage totals are unavailable; the response is showing a zeroed usage shell.",
			err,
		),
	}
	if strings.TrimSpace(project) != "" {
		diagnostics = append(diagnostics, buildDockerReadDiagnostic(
			dockerReadDiagnosticScopeUsageProjectCounts,
			dockerReadDiagnosticCodeProjectCountsLimited,
			"Project-scoped Docker usage counts are unavailable because Docker-backed inventory reads failed.",
			err,
		))
	}
	return diagnostics
}

func degradedDockerUsageSummary(project string) DockerUsageSummary {
	summary := DockerUsageSummary{
		TotalSize:  formatDockerBytes(0),
		Images:     DockerUsageEntry{Count: 0, Size: formatDockerBytes(0)},
		Containers: DockerUsageEntry{Count: 0, Size: formatDockerBytes(0)},
		Volumes:    DockerUsageEntry{Count: 0, Size: formatDockerBytes(0)},
		BuildCache: DockerUsageEntry{Count: 0, Size: formatDockerBytes(0)},
	}
	project = strings.TrimSpace(project)
	if project != "" {
		summary.Project = project
		summary.ProjectCounts = &DockerUsageProjectCounts{}
	}
	return summary
}

func DegradedDockerContainerDiagnostics(err error) []DockerReadDiagnostic {
	return degradedDockerContainerDiagnostics(err)
}

func DegradedDockerUsageDiagnostics(project string, err error) []DockerReadDiagnostic {
	return degradedDockerUsageDiagnostics(project, err)
}

func DegradedDockerUsageSummary(project string) DockerUsageSummary {
	return degradedDockerUsageSummary(project)
}

func IsDockerUsageProjectCountsDegraded(err error) bool {
	return isDockerUsageProjectCountsDegraded(err)
}

func wrapDockerUsageProjectCountsDegraded(err error) error {
	if err == nil {
		return nil
	}
	return dockerUsageProjectCountsDegradedError{cause: err}
}

func isDockerUsageProjectCountsDegraded(err error) bool {
	var degraded dockerUsageProjectCountsDegradedError
	return errors.As(err, &degraded)
}

func buildDockerReadDiagnostic(scope, code, message string, err error) DockerReadDiagnostic {
	return DockerReadDiagnostic{
		Scope:      scope,
		Status:     dockerReadDiagnosticStatusDegraded,
		Code:       code,
		Message:    message,
		SourceCode: dockerReadDiagnosticSourceCode(err),
		TaskType:   dockerReadDiagnosticTaskType(err),
	}
}

func dockerReadDiagnosticSourceCode(err error) string {
	for current := errors.Unwrap(err); current != nil; current = errors.Unwrap(current) {
		typed, ok := errs.From(current)
		if !ok {
			continue
		}
		code := strings.TrimSpace(string(typed.Code))
		if code != "" {
			return code
		}
	}

	typed, ok := errs.From(err)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(typed.Code))
}

func dockerReadDiagnosticTaskType(err error) string {
	for current := err; current != nil; current = errors.Unwrap(current) {
		typed, ok := errs.From(current)
		if !ok {
			continue
		}
		details, ok := typed.Details.(map[string]any)
		if !ok {
			continue
		}
		rawTaskType, exists := details["task_type"]
		if !exists {
			continue
		}
		switch value := rawTaskType.(type) {
		case contract.TaskType:
			return strings.TrimSpace(string(value))
		case string:
			return strings.TrimSpace(value)
		}
	}
	return ""
}
