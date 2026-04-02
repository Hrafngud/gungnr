package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type scriptedExecCall struct {
	dir  string
	env  []string
	name string
	args []string
}

type scriptedExecutor struct {
	calls []scriptedExecCall
	run   func(name string, args []string) ([]byte, error)
}

func (s *scriptedExecutor) Run(_ context.Context, req commandRequest) ([]byte, error) {
	copiedArgs := make([]string, len(req.Args))
	copy(copiedArgs, req.Args)
	copiedEnv := make([]string, len(req.Env))
	copy(copiedEnv, req.Env)
	s.calls = append(s.calls, scriptedExecCall{
		dir:  req.Dir,
		env:  copiedEnv,
		name: req.Name,
		args: copiedArgs,
	})
	if s.run == nil {
		return nil, fmt.Errorf("unexpected command: %s", req.Name)
	}
	return s.run(req.Name, copiedArgs)
}

func TestReadSystemImageAndKernelPrefersDockerDaemonIdentity(t *testing.T) {
	t.Parallel()

	exec := &scriptedExecutor{
		run: func(name string, args []string) ([]byte, error) {
			if name != "docker" || len(args) != 3 || args[0] != "info" || args[1] != "--format" {
				return nil, fmt.Errorf("unexpected command: %s %v", name, args)
			}
			switch args[2] {
			case "{{.OperatingSystem}}":
				return []byte("Host Linux 24.04\n"), nil
			case "{{.KernelVersion}}":
				return []byte("6.8.0-host\n"), nil
			default:
				return nil, fmt.Errorf("unexpected format: %s", args[2])
			}
		},
	}

	image, kernel, err := readSystemImageAndKernel(exec, context.Background())
	require.NoError(t, err)
	require.Equal(t, "Host Linux 24.04", image)
	require.Equal(t, "6.8.0-host", kernel)
	require.Len(t, exec.calls, 2)
}

func TestReadRootDiskUsageBytesUsesTemplatesPath(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	exec := &scriptedExecutor{
		run: func(name string, args []string) ([]byte, error) {
			if name != "df" {
				return nil, fmt.Errorf("unexpected command: %s", name)
			}
			require.Equal(t, []string{"-B1", templatesDir}, args)
			return []byte("Filesystem 1B-blocks Used Available Use% Mounted on\n/dev/sda1 1000 400 600 40% /mnt\n"), nil
		},
	}

	total, used, available, err := readRootDiskUsageBytes(context.Background(), exec, templatesDir)
	require.NoError(t, err)
	require.Equal(t, int64(1000), total)
	require.Equal(t, int64(400), used)
	require.Equal(t, int64(600), available)
	require.Len(t, exec.calls, 1)
}

func TestReadRootDiskUsageBytesFallsBackToRoot(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	callCount := 0
	exec := &scriptedExecutor{
		run: func(name string, args []string) ([]byte, error) {
			if name != "df" || len(args) != 2 || args[0] != "-B1" {
				return nil, fmt.Errorf("unexpected command: %s %v", name, args)
			}
			callCount++
			if callCount == 1 {
				require.Equal(t, templatesDir, args[1])
				return nil, errors.New("probe failed")
			}
			require.Equal(t, "/", args[1])
			return []byte("Filesystem 1B-blocks Used Available Use% Mounted on\n/dev/sda1 9000 1000 8000 11% /\n"), nil
		},
	}

	total, used, available, err := readRootDiskUsageBytes(context.Background(), exec, templatesDir)
	require.NoError(t, err)
	require.Equal(t, int64(9000), total)
	require.Equal(t, int64(1000), used)
	require.Equal(t, int64(8000), available)
	require.Len(t, exec.calls, 2)
}
