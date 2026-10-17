package service

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"go-notes/internal/infra/contract"
)

type templateEnvironmentStub struct {
	stubProjectFileMutationClient
	subnets []string
	err     error
}

func (s *templateEnvironmentStub) DockerNetworkSubnets(context.Context, string) (contract.Result, error) {
	return contract.Result{Status: contract.StatusSucceeded, Data: map[string]any{"lines": s.subnets}}, s.err
}

func TestEntGoVueEnvironmentCreatesConsistentUniqueCredentialsAndNetwork(t *testing.T) {
	base := t.TempDir()
	client := &templateEnvironmentStub{subnets: []string{"172.30.90.0/24", "172.30.91.0/24", "10.0.0.0/8"}}
	var first map[string]string
	for i, name := range []string{"mani", "another-app"} {
		dir := filepath.Join(base, name)
		require.NoError(t, os.Mkdir(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte(entGoVueExample), 0o644))
		require.NoError(t, PrepareEntGoVueEnvironment(context.Background(), client, dir, name, 18082+i, 55432+i))
		values, err := godotenv.Read(filepath.Join(dir, ".env"))
		require.NoError(t, err)
		u, err := url.Parse(values["DATABASE_URL"])
		require.NoError(t, err)
		password, _ := u.User.Password()
		require.Equal(t, values["POSTGRES_USER"], u.User.Username())
		require.Equal(t, values["POSTGRES_PASSWORD"], password)
		require.Equal(t, "/"+values["POSTGRES_DB"], u.Path)
		require.Equal(t, "postgres:5432", u.Host)
		require.Len(t, password, 64)
		require.Len(t, values["JWT_SECRET"], 64)
		require.Empty(t, values["ROOT_PASSWORD"])
		require.Equal(t, "512m", values["BACKEND_MEMORY_LIMIT"])
		require.NotContains(t, values["DATABASE_URL"], "replace_with")
		if i == 0 {
			first = values
			require.Equal(t, "172.30.92.0/24", values["APP_SUBNET"])
			require.Equal(t, "172.30.92.128/25", values["APP_IP_RANGE"])
			require.Equal(t, "172.30.92.2", values["NGINX_INTERNAL_IP"])
		} else {
			require.Equal(t, "172.30.93.0/24", values["APP_SUBNET"])
			require.NotEqual(t, first["JWT_SECRET"], values["JWT_SECRET"])
			require.NotEqual(t, first["POSTGRES_PASSWORD"], values["POSTGRES_PASSWORD"])
		}
		before, err := os.ReadFile(filepath.Join(dir, ".env"))
		require.NoError(t, err)
		require.NoError(t, PrepareEntGoVueEnvironment(context.Background(), client, dir, name, 19000, 55432))
		after, err := os.ReadFile(filepath.Join(dir, ".env"))
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
	for _, call := range client.writeCalls {
		require.Equal(t, uint32(0o600), call.Mode)
	}
}

func TestEntGoVueEnvironmentFailsWithoutNetworkInventoryOrWithSymlink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mani")
	require.NoError(t, os.Mkdir(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte(entGoVueExample), 0o644))
	client := &templateEnvironmentStub{err: errors.New("network inventory unavailable")}
	require.Error(t, PrepareEntGoVueEnvironment(context.Background(), client, dir, "mani", 18082, 5432))
	require.Empty(t, client.writeCalls)
	require.NoError(t, os.Symlink(filepath.Join(dir, ".env.example"), filepath.Join(dir, ".env")))
	require.ErrorContains(t, PrepareEntGoVueEnvironment(context.Background(), client, dir, "mani", 18082, 5432), "regular file")
	_, err := selectTemplateSubnet([]string{"172.30.0.0/16"})
	require.Error(t, err)
	logger := &captureWorkflowLogger{}
	require.NoError(t, prepareTemplateEnvironment(context.Background(), nil, logger, "Hrafngud", "go-ground", dir, "mani", 18082, 5432))
	require.Empty(t, logger.lines)
}

const entGoVueExample = `POSTGRES_DB=ent_go_vue
POSTGRES_USER=ent_go_vue
POSTGRES_PASSWORD=replace_with_random_database_password
DATABASE_URL=postgres://ent_go_vue:replace_with_random_database_password@postgres:5432/ent_go_vue?sslmode=disable
JWT_SECRET=replace_with_at_least_32_random_characters
ROOT_EMAIL=
ROOT_PASSWORD=
HTTP_PORT=18080
APP_SUBNET=172.30.90.0/24
APP_IP_RANGE=172.30.90.128/25
NGINX_INTERNAL_IP=172.30.90.2
BACKEND_MEMORY_LIMIT=512m
`
