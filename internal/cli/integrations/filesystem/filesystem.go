package filesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Paths struct {
	HomeDir        string
	CloudflaredDir string
	DataDir        string
}

type DataPaths struct {
	Root         string
	TemplatesDir string
	StateDir     string
	EnvPath      string
}

func DefaultPaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("unable to resolve home directory: %w", err)
	}
	if homeDir == "" {
		return Paths{}, errors.New("home directory is empty")
	}

	return Paths{
		HomeDir:        homeDir,
		CloudflaredDir: filepath.Join(homeDir, ".cloudflared"),
		DataDir:        filepath.Join(homeDir, "gungnr"),
	}, nil
}

func CheckExistingInstall(dataDir string) error {
	info, err := os.Stat(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to check Gungnr data directory %s: %w", dataDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("Gungnr data path %s exists but is not a directory", dataDir)
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("unable to inspect Gungnr data directory %s: %w", dataDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("existing Gungnr install detected at %s. Move or remove it before bootstrapping", dataDir)
	}

	return nil
}

func CheckDirAccess(label, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			parent := filepath.Dir(path)
			parentInfo, parentErr := os.Stat(parent)
			if parentErr != nil {
				return fmt.Errorf("%s missing at %s and unable to access parent %s: %w", label, path, parent, parentErr)
			}
			if !parentInfo.IsDir() {
				return fmt.Errorf("%s missing at %s and parent %s is not a directory", label, path, parent)
			}
			if !isWritable(parentInfo) {
				return fmt.Errorf("%s missing at %s and parent %s is not writable", label, path, parent)
			}
			return nil
		}
		return fmt.Errorf("unable to access %s at %s: %w", label, path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s at %s is not a directory", label, path)
	}

	if !isWritable(info) {
		return fmt.Errorf("%s at %s is not writable", label, path)
	}

	return nil
}

func PrepareDataDir(dataDir string) (DataPaths, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return DataPaths{}, fmt.Errorf("unable to create Gungnr data directory: %w", err)
	}

	templatesDir := filepath.Join(dataDir, "templates")
	stateDir := filepath.Join(dataDir, "state")
	for _, dir := range []string{templatesDir, stateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return DataPaths{}, fmt.Errorf("unable to create %s: %w", dir, err)
		}
	}

	envPath := filepath.Join(dataDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		return DataPaths{}, fmt.Errorf("bootstrap .env already exists at %s", envPath)
	} else if !os.IsNotExist(err) {
		return DataPaths{}, fmt.Errorf("unable to check %s: %w", envPath, err)
	}

	return DataPaths{
		Root:         dataDir,
		TemplatesDir: templatesDir,
		StateDir:     stateDir,
		EnvPath:      envPath,
	}, nil
}

func WriteEnvFile(path string, entries []EnvEntry) error {
	var builder strings.Builder
	for _, entry := range entries {
		if strings.TrimSpace(entry.Value) == "" {
			continue
		}
		builder.WriteString(entry.Key)
		builder.WriteString("=")
		builder.WriteString(formatEnvValue(entry.Value))
		builder.WriteString("\n")
	}

	return os.WriteFile(path, []byte(builder.String()), 0o600)
}

type EnvEntry struct {
	Key   string
	Value string
}

func formatEnvValue(value string) string {
	if value == "" {
		return value
	}
	if !strings.ContainsAny(value, " \t\r\n#\"'\\") {
		return value
	}
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", escaped)
}

func CopyFile(src, dest string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	return output.Sync()
}

func WaitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", path)
		}
		time.Sleep(2 * time.Second)
	}
}

func isWritable(info os.FileInfo) bool {
	mode := info.Mode().Perm()
	return mode&0o200 != 0 || mode&0o020 != 0 || mode&0o002 != 0
}
