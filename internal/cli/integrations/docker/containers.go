package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
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
