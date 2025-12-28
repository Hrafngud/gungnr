## Backend Guidelines (Go + Gin + GORM)

### Stack & Versions
- Go 1.25+ (module already `go-notes`).
- Gin v1.11+ (high-performance HTTP).
- GORM v2 (latest) with pgx/v5 (`gorm.io/driver/postgres` + `github.com/jackc/pgx/v5/stdlib`).
- Middleware: `github.com/gin-gonic/contrib` (CORS/logging) or `github.com/gin-contrib/*` equivalents when needed.
- Config: `github.com/spf13/viper` (preferred) or `github.com/joho/godotenv` fallback.
- Security utils: `golang.org/x/crypto/bcrypt` for hashing readiness.
- Testing: `net/http/httptest`, `github.com/stretchr/testify` for assertions.

### Architecture
- Layers: models → repositories (DB via GORM) → services (business logic) → controllers (Gin handlers) → routes.
- Keep DTOs/requests/responses separate from DB models to avoid accidental coupling.
- Use dependency injection via constructors; avoid global DB/routers except in main wiring.
- Transactions live in service layer; repositories expose granular operations.

### Setup (from repo root)
```bash
cd backend
go env -w GOPRIVATE=  # keep default; placeholder if needed
go get github.com/gin-gonic/gin@latest
go get gorm.io/gorm@latest gorm.io/driver/postgres@latest github.com/jackc/pgx/v5/stdlib@latest
go get github.com/gin-gonic/contrib@latest github.com/spf13/viper@latest github.com/joho/godotenv@latest
go get golang.org/x/crypto@latest github.com/stretchr/testify@latest
go mod tidy
```

### Configuration
- Preferred: `config/` with `config.go` loading via Viper from `.env` and environment variables.
- Sample env keys:
  - `APP_ENV=local|prod`
  - `PORT=8080`
  - `DATABASE_URL=postgres://user:pass@host:5432/notes?sslmode=disable`
  - `DB_MAX_OPEN_CONNS=20`, `DB_MAX_IDLE_CONNS=10`, `DB_CONN_MAX_LIFETIME_MIN=30`
- Boot flow: load config → connect DB (pgx driver) → auto-migrate models → build repositories/services/controllers → start Gin.

### Database & Migrations
- Use GORM auto-migrate in startup for Note model (id/title/content/timestamps) and future models.
- Keep `db/migrations.go` wrapper if we need ordered migrations later.
- Ensure `gorm.Model` or explicit fields for `CreatedAt`, `UpdatedAt`, `DeletedAt` (soft delete optional).

### HTTP API Patterns
- JSON only; request/response structs with validation.
- Middlewares: CORS (allow frontend origin), request logging, recovery.
- Routing: `/api/v1/notes` with CRUD; health at `/healthz`.
- Error handling: consistent `{"error":"message"}` responses; use `context.AbortWithStatusJSON`.

### Testing
- Unit-test services with repository interfaces + fakes.
- Integration-test handlers with `httptest` and a transient test DB (or SQLite in-memory when behavior matches).
- Add minimal coverage for create/list/update/delete paths.

### Local Run
```bash
cd backend
go run ./cmd/server
```
- Ensure `.env` or exported vars present; log startup info (port/env).

### Docker
- Dockerfile: multi-stage (builder → tiny runtime, e.g., `alpine:3.19`), CGO_ENABLED=0 for static build unless we rely on CGO in pgx (pgx works without CGO).
- Entrypoint should wait for DB (simple retry script) or rely on Compose healthchecks.
