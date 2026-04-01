package docker

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gungnr-cli/internal/cli/integrations/command"
)

func CheckDockerAccess() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker not found in PATH. Install Docker and retry")
	}

	if _, err := command.Run("docker", "info"); err != nil {
		return fmt.Errorf("docker access failed: %w", err)
	}

	return nil
}

func CheckCompose() error {
	if _, err := command.Run("docker", "compose", "version"); err == nil {
		return nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := command.Run("docker-compose", "version"); runErr == nil {
			return nil
		}
	}

	return errors.New("docker compose not available. Install Docker Compose v2 (docker compose) or docker-compose")
}

func ResolveComposeCommand() (string, []string, error) {
	if _, err := command.Run("docker", "compose", "version"); err == nil {
		return "docker", []string{"compose"}, nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := command.Run("docker-compose", "version"); runErr == nil {
			return "docker-compose", nil, nil
		}
	}

	return "", nil, errors.New("docker compose not available")
}

func FindComposeFile() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to resolve working directory: %w", err)
	}
	return FindComposeFileFromDir(startDir)
}

func FindComposeFileFromDir(startDir string) (string, error) {
	dir := startDir
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("compose search directory is empty")
	}
	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}

	for {
		composePath := filepath.Join(dir, "docker-compose.yml")
		if info, err := os.Stat(composePath); err == nil && !info.IsDir() {
			return composePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("docker-compose.yml not found from %s upward; run bootstrap from the repo root", startDir)
}

func DockerSocketGID() (string, error) {
	info, err := os.Stat("/var/run/docker.sock")
	if err != nil {
		return "", fmt.Errorf("unable to stat /var/run/docker.sock: %w", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("unable to resolve docker socket group id")
	}
	return strconv.FormatUint(uint64(stat.Gid), 10), nil
}

func StartCompose(composeFile, envFile, logPath string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args, err := composeArgs(baseArgs, composeFile, envFile)
	if err != nil {
		return err
	}
	args = append(args, "up", "-d", "--build")
	return command.RunLoggedInDir(composeDir, commandName, logPath, args...)
}

func RebuildCompose(composeFile, envFile, logPath string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args, err := composeArgs(baseArgs, composeFile, envFile)
	if err != nil {
		return err
	}
	args = append(args, "up", "--build", "--force-recreate", "-d")
	return command.RunLoggedInDir(composeDir, commandName, logPath, args...)
}

func EnsureComposeRunning(composeFile, envFile, logPath string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args, err := composeArgs(baseArgs, composeFile, envFile)
	if err != nil {
		return err
	}
	args = append(args, "up", "-d")
	return command.RunLoggedInDir(composeDir, commandName, logPath, args...)
}

func StopCompose(composeFile, envFile string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args, err := composeArgs(baseArgs, composeFile, envFile)
	if err != nil {
		return err
	}
	args = append(args, "down")
	return command.RunInteractiveInDir(composeDir, commandName, args...)
}

func composeArgs(baseArgs []string, composeFile, envFile string) ([]string, error) {
	args := append([]string{}, baseArgs...)
	args = append(args, "--env-file", envFile, "-f", composeFile)

	mode, err := dockerNetworkModeFromEnvFile(envFile)
	if err != nil {
		return nil, err
	}
	if mode == "compat" {
		compatFile := filepath.Join(filepath.Dir(composeFile), "docker-compose.network-compat.yml")
		if info, statErr := os.Stat(compatFile); statErr == nil && !info.IsDir() {
			args = append(args, "-f", compatFile)
		}
	}
	return args, nil
}

func dockerNetworkModeFromEnvFile(envFile string) (string, error) {
	file, err := os.Open(envFile)
	if err != nil {
		return "", fmt.Errorf("unable to open env file %s: %w", envFile, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "DOCKER_NETWORK_GUARDRAILS_MODE=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "DOCKER_NETWORK_GUARDRAILS_MODE="))
		return strings.Trim(value, "\""), nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("unable to read env file %s: %w", envFile, err)
	}
	return "", nil
}
