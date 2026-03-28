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
		require.Contains(t, content, "DOCKER_DAEMON_ISOLATION_MODE: ${DOCKER_DAEMON_ISOLATION_MODE:-disabled}", "%s must default daemon isolation selection to disabled", filepath.Base(path))
		require.Contains(t, content, "rollback is always DOCKER_DAEMON_ISOLATION_MODE=disabled", "%s must document deterministic daemon isolation rollback selection", filepath.Base(path))
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

func TestComposeFilesApplyHardeningProfileWithDocumentedExceptions(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	composeFiles := []string{
		filepath.Join(repoRoot, "docker-compose.yml"),
		filepath.Join(repoRoot, "docker-compose.release.yml"),
	}

	for _, path := range composeFiles {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(contentBytes)

		dbSection := composeServiceSection(t, content, "db", "api")
		require.Contains(t, dbSection, "# Hardening exception: Postgres retains only ownership/user-switch capabilities plus data/runtime-write paths.", "%s must document the Postgres exception set", filepath.Base(path))
		require.Contains(t, dbSection, "security_opt:\n      - no-new-privileges:true", "%s db must enable no-new-privileges", filepath.Base(path))
		require.Contains(t, dbSection, "cap_drop:\n      - ALL", "%s db must drop default capabilities", filepath.Base(path))
		require.Contains(t, dbSection, "cap_add:\n      - CHOWN\n      - DAC_READ_SEARCH\n      - FOWNER\n      - SETGID\n      - SETUID", "%s db must retain only the minimal Postgres ownership/user-switch capabilities", filepath.Base(path))
		require.Contains(t, dbSection, "read_only: true", "%s db must keep the root filesystem read-only", filepath.Base(path))
		require.Contains(t, dbSection, "tmpfs:\n      - /tmp\n      - /var/run/postgresql", "%s db must keep only runtime tmpfs paths writable", filepath.Base(path))
		require.Contains(t, dbSection, "- pgdata:/var/lib/postgresql/data", "%s db must keep the explicit data volume mount", filepath.Base(path))
		require.Contains(t, dbSection, "pids_limit: 256", "%s db must keep deterministic PID guardrails", filepath.Base(path))

		apiSection := composeServiceSection(t, content, "api", "web")
		require.Contains(t, apiSection, "# Hardening exception: runtime host integrations keep mounted templates/cloudflared/socket paths writable.", "%s api must document its writable mount exception", filepath.Base(path))
		require.Contains(t, apiSection, "security_opt:\n      - no-new-privileges:true", "%s api must enable no-new-privileges", filepath.Base(path))
		require.Contains(t, apiSection, "cap_drop:\n      - ALL", "%s api must drop default capabilities", filepath.Base(path))
		require.Contains(t, apiSection, "read_only: true", "%s api must keep the root filesystem read-only", filepath.Base(path))
		require.Contains(t, apiSection, "tmpfs:\n      - /tmp", "%s api must keep only /tmp as writable tmpfs", filepath.Base(path))
		require.Contains(t, apiSection, "- /var/run/docker.sock:/var/run/docker.sock", "%s api must preserve the explicit docker socket mount", filepath.Base(path))
		require.Contains(t, apiSection, "- ${TEMPLATES_DIR:-/templates}:${TEMPLATES_DIR:-/templates}", "%s api must preserve the explicit templates mount", filepath.Base(path))
		require.Contains(t, apiSection, "- ${CLOUDFLARED_DIR:-/home/user/.cloudflared}:${CLOUDFLARED_DIR:-/home/user/.cloudflared}", "%s api must preserve the explicit cloudflared mount", filepath.Base(path))
		require.Contains(t, apiSection, "pids_limit: 256", "%s api must keep deterministic PID guardrails", filepath.Base(path))
		require.NotContains(t, apiSection, "cap_add:", "%s api must not retain extra capabilities", filepath.Base(path))

		webSection := composeServiceSection(t, content, "web", "proxy")
		require.Contains(t, webSection, "# Hardening exception: nginx retains bind/user-switch capabilities plus tmpfs runtime/cache paths.", "%s web must document the nginx capability exception", filepath.Base(path))
		require.Contains(t, webSection, "security_opt:\n      - no-new-privileges:true", "%s web must enable no-new-privileges", filepath.Base(path))
		require.Contains(t, webSection, "cap_drop:\n      - ALL", "%s web must drop default capabilities", filepath.Base(path))
		require.Contains(t, webSection, "cap_add:\n      - NET_BIND_SERVICE\n      - SETGID\n      - SETUID", "%s web must retain only the minimal nginx bind/user-switch capabilities", filepath.Base(path))
		require.Contains(t, webSection, "read_only: true", "%s web must keep the root filesystem read-only", filepath.Base(path))
		require.Contains(t, webSection, "tmpfs:\n      - /var/cache/nginx\n      - /var/run\n      - /tmp", "%s web must keep only nginx runtime/cache tmpfs paths writable", filepath.Base(path))
		require.Contains(t, webSection, "pids_limit: 64", "%s web must keep deterministic PID guardrails", filepath.Base(path))

		proxySection := composeServiceSection(t, content, "proxy", "networks")
		require.Contains(t, proxySection, "# Hardening exception: nginx retains bind/user-switch capabilities plus tmpfs runtime/cache paths.", "%s proxy must document the nginx capability exception", filepath.Base(path))
		require.Contains(t, proxySection, "security_opt:\n      - no-new-privileges:true", "%s proxy must enable no-new-privileges", filepath.Base(path))
		require.Contains(t, proxySection, "cap_drop:\n      - ALL", "%s proxy must drop default capabilities", filepath.Base(path))
		require.Contains(t, proxySection, "cap_add:\n      - NET_BIND_SERVICE\n      - SETGID\n      - SETUID", "%s proxy must retain only the minimal nginx bind/user-switch capabilities", filepath.Base(path))
		require.Contains(t, proxySection, "read_only: true", "%s proxy must keep the root filesystem read-only", filepath.Base(path))
		require.Contains(t, proxySection, "tmpfs:\n      - /var/cache/nginx\n      - /var/run\n      - /tmp", "%s proxy must keep only nginx runtime/cache tmpfs paths writable", filepath.Base(path))
		require.Contains(t, proxySection, "pids_limit: 64", "%s proxy must keep deterministic PID guardrails", filepath.Base(path))
	}
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

func TestQuickServiceWorkerPolicyRemainsHardenedAndLoopbackPublished(t *testing.T) {
	repoRoot := repoRootFromServiceTest(t)
	path := filepath.Join(repoRoot, "backend", "internal", "infra", "worker", "worker.go")

	contentBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(contentBytes)

	require.Contains(t, content, `"network", "inspect", networkName`, "quick-service worker must resolve the managed runtime network before starting containers")
	require.Contains(t, content, `func (r *Runner) ensureQuickServiceNetwork`, "quick-service worker must keep managed-network creation logic in a dedicated helper")
	require.Contains(t, content, `"--network"`, "quick-service worker must attach containers to an explicit managed network")
	require.Contains(t, content, `"--security-opt"`, "quick-service worker must keep no-new-privileges enabled")
	require.Contains(t, content, `"no-new-privileges:true"`, "quick-service worker must pin no-new-privileges")
	require.Contains(t, content, `"--cap-drop"`, "quick-service worker must drop default capabilities")
	require.Contains(t, content, `"--cap-add", "NET_BIND_SERVICE"`, "quick-service worker must retain NET_BIND_SERVICE for privileged container ports")
	require.Contains(t, content, `"--pids-limit"`, "quick-service worker must keep deterministic PID bounds")
	require.Contains(t, content, `"--memory"`, "quick-service worker must keep deterministic memory bounds")
	require.Contains(t, content, `"--cpus"`, "quick-service worker must keep deterministic CPU bounds")
	require.Contains(t, content, `contract.QuickServiceNetworkLabelKey`, "quick-service worker must label managed runtime networks explicitly")
	require.Contains(t, content, `fmt.Sprintf("%s:%d:%d", payload.PublishHost, payload.HostPort, payload.ContainerPort)`, "quick-service worker must keep host publish formatting explicit")
	require.Contains(t, content, `contract.NormalizeQuickServicePublishHost`, "quick-service worker must normalize publish host through the shared loopback-only guard")
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

func composeServiceSection(t *testing.T, content, service, next string) string {
	t.Helper()

	startMarker := "  " + service + ":\n"
	start := strings.Index(content, startMarker)
	require.NotEqualf(t, -1, start, "compose must include %s service", service)

	var endMarker string
	if next == "networks" {
		endMarker = "\nnetworks:\n"
	} else {
		endMarker = "\n  " + next + ":\n"
	}

	searchFrom := start + len(startMarker)
	endOffset := strings.Index(content[searchFrom:], endMarker)
	require.NotEqualf(t, -1, endOffset, "%s service must precede %s", service, next)

	return content[start : searchFrom+endOffset]
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
