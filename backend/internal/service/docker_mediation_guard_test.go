package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceAndControllerRuntimePathsDoNotExecDockerDirectly(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	serviceDir := filepath.Join(repoRoot, "backend", "internal", "service")
	controllerDir := filepath.Join(repoRoot, "backend", "internal", "controller")

	var violations []string
	violations = append(violations, findDirectDockerExecViolations(t, serviceDir)...)
	violations = append(violations, findDirectDockerExecViolations(t, controllerDir)...)

	require.Empty(t, violations, "direct docker/docker-compose runtime execution must stay infra-bridge mediated:\n%s", strings.Join(violations, "\n"))
}

func TestComposeFilesNarrowAPIMountScope(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	composeFiles := []string{
		filepath.Join(repoRoot, "docker-compose.yml"),
		filepath.Join(repoRoot, "docker-compose.release.yml"),
	}

	for _, path := range composeFiles {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(contentBytes)

		require.NotContains(t, content, "HOST_HOME_DIR", "%s should not include broad host-home bind mounts", filepath.Base(path))
		require.Contains(t, content, "/var/run/docker.sock:/var/run/docker.sock", "%s must keep worker docker-socket access", filepath.Base(path))
		require.Contains(t, content, "${CLOUDFLARED_DIR", "%s must keep explicit cloudflared directory mount", filepath.Base(path))
	}
}

func TestComposeFilesDisableDBHostPublishByDefault(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	composeFiles := []string{
		filepath.Join(repoRoot, "docker-compose.yml"),
		filepath.Join(repoRoot, "docker-compose.release.yml"),
	}

	for _, path := range composeFiles {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(contentBytes)

		require.NotContains(t, content, "\"5432:5432\"", "%s should not publish postgres on host by default", filepath.Base(path))
		require.Contains(t, content, "DB_HOST_PUBLISH_MODE: ${DB_HOST_PUBLISH_MODE:-disabled}", "%s must default DB host publish mode to disabled", filepath.Base(path))
		require.Contains(t, content, "DB_HOST_PUBLISH_HOST: ${DB_HOST_PUBLISH_HOST:-127.0.0.1}", "%s must keep loopback-only host publish bind default", filepath.Base(path))
		require.Contains(t, content, "DB_HOST_PUBLISH_PORT: ${DB_HOST_PUBLISH_PORT:-5432}", "%s must keep deterministic DB host publish port diagnostics", filepath.Base(path))
	}
}

func TestComposeLoopbackDBPublishOverrideIsLoopbackOnly(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	path := filepath.Join(repoRoot, "docker-compose.db-host-loopback.yml")

	contentBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(contentBytes)

	require.Contains(t, content, "127.0.0.1:${DB_HOST_PUBLISH_PORT:-5432}:5432", "DB host publish override must bind loopback only")
	require.Contains(t, content, "DB_HOST_PUBLISH_MODE: loopback", "override must surface loopback mode in runtime diagnostics")
	require.Contains(t, content, "DB_HOST_PUBLISH_HOST: 127.0.0.1", "override must surface loopback host in runtime diagnostics")
}

func TestComposeNetworkCompatOverrideDisablesICCGuardrailsWithoutChangingPlaneNames(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	path := filepath.Join(repoRoot, "docker-compose.network-compat.yml")

	contentBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(contentBytes)

	require.Contains(t, content, "DOCKER_NETWORK_GUARDRAILS_MODE: compat", "compat override must align runtime diagnostics with effective network mode")
	require.Contains(t, content, "edge:\n    driver: bridge", "compat override must redefine edge network without hardened driver opts")
	require.Contains(t, content, "core:\n    driver: bridge\n    internal: true", "compat override must keep core internal while removing hardened driver opts")
	require.NotContains(t, content, "com.docker.network.bridge.enable_icc", "compat override must remove ICC driver opts for unsupported engines")
	require.NotContains(t, content, "edge_compat", "compat override must keep canonical plane names to avoid topology/reporting drift")
	require.NotContains(t, content, "core_compat", "compat override must keep canonical plane names to avoid topology/reporting drift")
}

func TestComposeFilesSplitNetworkPlanesWithDeterministicCompatFallback(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	composeFiles := []string{
		filepath.Join(repoRoot, "docker-compose.yml"),
		filepath.Join(repoRoot, "docker-compose.release.yml"),
	}

	for _, path := range composeFiles {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(contentBytes)

		require.NotContains(t, content, "app-network", "%s should not keep a flat shared network topology", filepath.Base(path))
		require.Contains(t, content, "DOCKER_NETWORK_GUARDRAILS_MODE: ${DOCKER_NETWORK_GUARDRAILS_MODE:-enforced}", "%s must surface deterministic guardrail mode to api runtime diagnostics", filepath.Base(path))
		require.Contains(t, content, "docker compose -f "+filepath.Base(path)+" -f docker-compose.network-compat.yml up -d", "%s must document explicit compat override handling", filepath.Base(path))

		require.Contains(t, content, "edge:\n    driver: bridge\n    driver_opts:\n      com.docker.network.bridge.enable_icc: \"false\"", "%s must enforce ICC guardrail on hardened edge network", filepath.Base(path))
		require.Contains(t, content, "core:\n    driver: bridge\n    internal: true\n    driver_opts:\n      com.docker.network.bridge.enable_icc: \"false\"", "%s must enforce ICC guardrail on hardened internal core network", filepath.Base(path))
		require.NotContains(t, content, "edge_compat", "%s must not expose alternate network names that can drift from actual topology", filepath.Base(path))
		require.NotContains(t, content, "core_compat", "%s must not expose alternate network names that can drift from actual topology", filepath.Base(path))
		require.Equal(t, 4, strings.Count(content, "- core"), "%s must place db/api/web/proxy on core plane", filepath.Base(path))
		require.Equal(t, 1, strings.Count(content, "- edge"), "%s must place only proxy on edge plane", filepath.Base(path))
		require.Contains(t, content, "- \"80:80\"", "%s must keep proxy as the single host-exposed edge entrypoint", filepath.Base(path))
	}
}

func findDirectDockerExecViolations(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	fset := token.NewFileSet()
	var violations []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		node, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)

		execAliases := map[string]struct{}{}
		for _, imp := range node.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			if importPath != "os/exec" {
				continue
			}

			alias := "exec"
			if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
				alias = imp.Name.Name
			}
			execAliases[alias] = struct{}{}
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if command, ok := dockerCommandFromExecCall(call, execAliases); ok {
				pos := fset.Position(call.Pos())
				violations = append(violations, pos.String()+": direct "+command+" execution via os/exec")
			}

			if command, ok := dockerCommandFromCommandHelperCall(call); ok {
				pos := fset.Position(call.Pos())
				violations = append(violations, pos.String()+": direct "+command+" execution via command helper")
			}

			return true
		})
	}

	return violations
}

func dockerCommandFromExecCall(call *ast.CallExpr, execAliases map[string]struct{}) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	x, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if _, exists := execAliases[x.Name]; !exists {
		return "", false
	}

	argIndex := -1
	switch selector.Sel.Name {
	case "Command":
		argIndex = 0
	case "CommandContext":
		argIndex = 1
	default:
		return "", false
	}

	if len(call.Args) <= argIndex {
		return "", false
	}
	return dockerLiteral(call.Args[argIndex])
}

func dockerCommandFromCommandHelperCall(call *ast.CallExpr) (string, bool) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return "", false
	}
	if ident.Name != "runLoggedCommand" && ident.Name != "runQuietCommand" {
		return "", false
	}

	const commandNameIndex = 4
	if len(call.Args) <= commandNameIndex {
		return "", false
	}
	return dockerLiteral(call.Args[commandNameIndex])
}

func dockerLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	switch strings.TrimSpace(strings.ToLower(value)) {
	case "docker", "docker-compose":
		return value, true
	default:
		return "", false
	}
}

func repoRootFromServiceTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
