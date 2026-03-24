package docker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gungnr-cli/internal/cli/integrations/command"
)

type ComposeContainer struct {
	ID      string
	Name    string
	Status  string
	Project string
	Service string
}

type ComposeProject struct {
	Name            string
	WorkingDir      string
	ConfigFiles     []string
	SeedContainerID string
}

type dockerPSLine struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Status string `json:"Status"`
	Labels string `json:"Labels"`
}

func ListComposeContainers(includeAll bool) ([]ComposeContainer, error) {
	args := []string{"ps"}
	if includeAll {
		args = append(args, "-a")
	}
	args = append(args, "--format", "{{json .}}")

	output, err := command.Run("docker", args...)
	if err != nil {
		return nil, fmt.Errorf("list docker containers: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return []ComposeContainer{}, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	containers := make([]ComposeContainer, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry dockerPSLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse docker ps entry: %w", err)
		}

		labels := parseDockerLabels(entry.Labels)
		project := strings.TrimSpace(labels["com.docker.compose.project"])
		if project == "" {
			continue
		}

		containers = append(containers, ComposeContainer{
			ID:      strings.TrimSpace(entry.ID),
			Name:    strings.TrimSpace(entry.Names),
			Status:  strings.TrimSpace(entry.Status),
			Project: project,
			Service: strings.TrimSpace(labels["com.docker.compose.service"]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan docker ps output: %w", err)
	}

	return containers, nil
}

func DiscoverComposeProjects(includeAll bool) ([]ComposeProject, error) {
	containers, err := ListComposeContainers(includeAll)
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return []ComposeProject{}, nil
	}

	projectSeedContainer := make(map[string]string)
	for _, container := range containers {
		project := strings.TrimSpace(container.Project)
		if project == "" {
			continue
		}
		if _, exists := projectSeedContainer[project]; exists {
			continue
		}
		projectSeedContainer[project] = strings.TrimSpace(container.ID)
	}

	projectNames := make([]string, 0, len(projectSeedContainer))
	for name := range projectSeedContainer {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	projects := make([]ComposeProject, 0, len(projectNames))
	for _, name := range projectNames {
		project, err := inspectComposeProject(projectSeedContainer[name], name)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func RebuildComposeProject(project ComposeProject, logPath string) error {
	return RebuildComposeProjectWithTimeout(project, logPath, 0)
}

func RebuildComposeProjectWithTimeout(project ComposeProject, logPath string, timeout time.Duration) error {
	if strings.TrimSpace(project.Name) == "" {
		return errors.New("compose project name is empty")
	}
	if len(project.ConfigFiles) == 0 {
		return fmt.Errorf("compose project %q has no compose config files", project.Name)
	}

	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := strings.TrimSpace(project.WorkingDir)
	if composeDir == "" {
		composeDir = filepath.Dir(project.ConfigFiles[0])
	}

	args := append([]string{}, baseArgs...)
	args = append(args, "-p", project.Name)
	for _, configFile := range project.ConfigFiles {
		args = append(args, "-f", configFile)
	}
	args = append(args, "up", "--build", "--force-recreate", "-d")

	return command.RunLoggedInDirWithTimeout(composeDir, commandName, logPath, timeout, args...)
}

func StartContainers(containerIDs []string) error {
	return startContainers(containerIDs, 0)
}

func StartContainersWithTimeout(containerIDs []string, timeout time.Duration) error {
	return startContainers(containerIDs, timeout)
}

func startContainers(containerIDs []string, timeout time.Duration) error {
	ids := dedupeContainerIDs(containerIDs)
	if len(ids) == 0 {
		return nil
	}

	args := append([]string{"start"}, ids...)
	if _, err := command.RunWithTimeout(timeout, "docker", args...); err != nil {
		return fmt.Errorf("start docker containers: %w", err)
	}
	return nil
}

func dedupeContainerIDs(containerIDs []string) []string {
	seen := make(map[string]struct{}, len(containerIDs))
	result := make([]string, 0, len(containerIDs))
	for _, id := range containerIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func parseDockerLabels(raw string) map[string]string {
	labels := make(map[string]string)
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		labels[key] = value
	}
	return labels
}

func inspectComposeProject(containerID, fallbackProject string) (ComposeProject, error) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return ComposeProject{}, fmt.Errorf("compose project %q has no seed container", fallbackProject)
	}

	output, err := command.Run("docker", "inspect", "--format", "{{json .Config.Labels}}", containerID)
	if err != nil {
		return ComposeProject{}, fmt.Errorf("inspect compose project %q: %w", fallbackProject, err)
	}

	labels := map[string]string{}
	trimmed := strings.TrimSpace(output)
	if trimmed != "" && trimmed != "<no value>" {
		if err := json.Unmarshal([]byte(trimmed), &labels); err != nil {
			return ComposeProject{}, fmt.Errorf("parse docker labels for project %q: %w", fallbackProject, err)
		}
	}

	projectName := strings.TrimSpace(labels["com.docker.compose.project"])
	if projectName == "" {
		projectName = strings.TrimSpace(fallbackProject)
	}
	if projectName == "" {
		return ComposeProject{}, errors.New("compose project name is empty")
	}

	workingDir := strings.TrimSpace(labels["com.docker.compose.project.working_dir"])
	configFiles := resolveComposeConfigFiles(workingDir, labels["com.docker.compose.project.config_files"])
	if len(configFiles) == 0 {
		return ComposeProject{}, fmt.Errorf("unable to resolve compose config files for project %q", projectName)
	}

	return ComposeProject{
		Name:            projectName,
		WorkingDir:      workingDir,
		ConfigFiles:     configFiles,
		SeedContainerID: containerID,
	}, nil
}

func resolveComposeConfigFiles(workingDir, raw string) []string {
	candidates := splitComposeConfigFiles(raw)
	if len(candidates) == 0 {
		candidates = []string{"docker-compose.yml", "compose.yml", "compose.yaml"}
	}

	seen := make(map[string]struct{})
	resolved := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}

		path := trimmed
		if !filepath.IsAbs(path) && strings.TrimSpace(workingDir) != "" {
			path = filepath.Join(workingDir, path)
		}

		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		resolved = append(resolved, path)
	}

	return resolved
}

func splitComposeConfigFiles(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
