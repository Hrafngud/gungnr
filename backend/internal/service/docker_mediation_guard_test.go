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
