package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-notes/internal/errs"
)

func TestWorkbenchSelectIngressPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		compose string
		service string
		port    int
		issue   string
	}{
		{
			name: "nginx 8080 with internal backend and database",
			compose: `services:
  nginx:
    ports: ["127.0.0.1:${HTTP_PORT:-18080}:8080"]
  backend:
    expose: [8080]
  postgres:
    image: postgres:17
    ports: ["55432:5432"]
`,
			service: "nginx", port: 8080,
		},
		{
			name: "proxy takes priority over nginx frontend",
			compose: `services:
  proxy:
    image: nginx:alpine
    ports: ["${PROXY_PORT:-80}:80"]
  web:
    image: nginx:alpine
    ports: ["8081:80"]
`,
			service: "proxy", port: 80,
		},
		{
			name: "custom proxy container port takes priority over frontend 80",
			compose: `services:
  proxy:
    ports: ["18080:9001"]
  frontend:
    ports: ["8081:80"]
`,
			service: "proxy", port: 9001,
		},
		{
			name: "proxy image identifies custom service name",
			compose: `services:
  router:
    image: caddy:2
    ports: ["18080:8080"]
  web:
    ports: ["8081:80"]
`,
			service: "router", port: 8080,
		},
		{
			name: "sole application TCP port",
			compose: `services:
  app:
    ports: ["18080:3000"]
  db:
    ports: ["55432:5432"]
`,
			service: "app", port: 3000,
		},
		{
			name: "HTTP preferred over HTTPS on same ingress",
			compose: `services:
  proxy:
    ports: ["18080:80", "18443:443"]
`,
			service: "proxy", port: 80,
		},
		{
			name: "UDP ignored",
			compose: `services:
  proxy:
    ports: ["18080:8080/udp"]
  app:
    ports: ["18081:3000/tcp"]
`,
			service: "app", port: 3000,
		},
		{
			name: "internal expose is not a published port",
			compose: `services:
  nginx:
    expose: [8080]
`,
			issue: "WB-JOB-INGRESS-PORT-MISSING",
		},
		{
			name: "database alone is not ingress",
			compose: `services:
  data:
    image: postgres:17
    ports: ["55432:5432"]
`,
			issue: "WB-JOB-INGRESS-PORT-MISSING",
		},
		{
			name: "ambiguous application ports",
			compose: `services:
  api:
    ports: ["18080:8080"]
  app:
    ports: ["18081:3000"]
`,
			issue: "WB-JOB-INGRESS-PORT-AMBIGUOUS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParseWorkbenchComposeCore(tt.compose)
			if err != nil {
				t.Fatal(err)
			}
			selector, err := workbenchSelectIngressPort(snapshotFromParsedCompose(parsed))
			if tt.issue != "" {
				typed, ok := errs.From(err)
				if !ok || typed.Code != errs.CodeWorkbenchValidationFailed {
					t.Fatalf("expected typed validation failure, got %v", err)
				}
				details, ok := typed.Details.(map[string]any)
				if !ok {
					t.Fatalf("expected structured selection diagnostics, got %#v", typed.Details)
				}
				issues, ok := details["issues"].([]WorkbenchValidationIssue)
				if !ok || len(issues) != 1 || issues[0].Code != tt.issue {
					t.Fatalf("expected issue %s, got %#v", tt.issue, typed.Details)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if selector.ServiceName != tt.service || selector.ContainerPort != tt.port || selector.Protocol != "tcp" {
				t.Fatalf("unexpected ingress selector: %#v", selector)
			}
		})
	}
}

func TestProjectWorkflowsPrepareWorkbenchManagedComposeDetectsIngress8080(t *testing.T) {
	t.Parallel()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	compose := `services:
  nginx:
    build: ./docker/nginx
    ports: ["127.0.0.1:${HTTP_PORT:-18080}:8080"]
    depends_on:
      backend:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/ > /dev/null"]
  backend:
    build: ./backend
    environment:
      DATABASE_URL: ${DATABASE_URL:?Set DATABASE_URL}
  web:
    image: nginx:alpine
    ports: ["8081:80"]
`
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	workflows := &ProjectWorkflows{workbench: svc}
	logger := &captureWorkflowLogger{}
	_, err := workflows.prepareWorkbenchManagedCompose(context.Background(), logger, "demo", workbenchImportReasonAutoDeploy, []workbenchRequestedPortAssignment{
		{Label: "proxy", DetectIngress: true, HostPort: 19000, Required: true},
		{Label: "db", ContainerPort: 5432, HostPort: 15432, Required: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseWorkbenchComposeCore(string(updated))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Ports) != 2 {
		t.Fatalf("unexpected published port count: %#v", parsed.Ports)
	}
	for _, port := range parsed.Ports {
		switch port.ServiceName {
		case "nginx":
			if port.HostPort == nil || *port.HostPort != 19000 || port.ContainerPort != 8080 || port.HostIP != "127.0.0.1" {
				t.Fatalf("ingress must preserve target port and loopback bind: %#v", port)
			}
		case "web":
			if port.HostPort == nil || *port.HostPort != 8081 || port.ContainerPort != 80 {
				t.Fatalf("frontend mapping must remain untouched: %#v", port)
			}
		}
	}
	for _, want := range []string{"http://127.0.0.1:8080/", "service_healthy", "${DATABASE_URL:?Set DATABASE_URL}"} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("compose lost %q: %s", want, updated)
		}
	}
	if !strings.Contains(strings.Join(logger.lines, "\n"), "ingress detected: service=nginx container=8080 protocol=tcp") {
		t.Fatalf("missing ingress detection diagnostic: %#v", logger.lines)
	}
}

func TestProjectWorkflowsPrepareWorkbenchManagedComposeRejectsAmbiguousIngressWithoutWriting(t *testing.T) {
	t.Parallel()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	compose := "services:\n  proxy:\n    ports: [\"18080:8080\", \"18081:8081\"]\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	workflows := &ProjectWorkflows{workbench: svc}
	_, err := workflows.prepareWorkbenchManagedCompose(context.Background(), &captureWorkflowLogger{}, "demo", workbenchImportReasonAutoDeploy, []workbenchRequestedPortAssignment{
		{Label: "proxy", DetectIngress: true, HostPort: 19000, Required: true},
	})
	if err == nil || !strings.Contains(err.Error(), "multiple possible ingress mappings") {
		t.Fatalf("expected an ambiguity error, got %v", err)
	}
	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != compose {
		t.Fatalf("ambiguous detection must not rewrite compose: %s", updated)
	}
}
