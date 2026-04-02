package docker

import (
	"testing"
)

func TestValidateDockerSocketGroupValue(t *testing.T) {
	t.Run("accepts matching gid", func(t *testing.T) {
		if err := validateDockerSocketGroupValue("952", "952"); err != nil {
			t.Fatalf("expected matching gid to pass, got %v", err)
		}
	})

	t.Run("rejects missing configured gid", func(t *testing.T) {
		err := validateDockerSocketGroupValue("", "952")
		if err == nil || err.Error() != "DOCKER_SOCKET_GID is missing from the panel .env; rerun `gungnr restart` to refresh the hardened runtime contract" {
			t.Fatalf("expected missing DOCKER_SOCKET_GID error, got %v", err)
		}
	})

	t.Run("rejects stale configured gid", func(t *testing.T) {
		err := validateDockerSocketGroupValue("0", "952")
		if err == nil || err.Error() != "DOCKER_SOCKET_GID=0 does not match the current /var/run/docker.sock group id 952; rerun `gungnr restart` to refresh the hardened runtime contract" {
			t.Fatalf("expected stale DOCKER_SOCKET_GID error, got %v", err)
		}
	})
}
