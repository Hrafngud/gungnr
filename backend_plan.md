## Backend Plan

Status: Note CRUD implemented (repo/service/controllers, routes, validation, tests) and Dockerized API (multi-stage image + compose wiring); remaining polish is observability/DX.

1) Scaffolding
- Create folders: `cmd/server`, `internal/config`, `internal/db`, `internal/models`, `internal/repository`, `internal/service`, `internal/controller`, `internal/router`, `internal/middleware`.
- Add main entry in `cmd/server/main.go`.

2) Dependencies & Config
- Add Gin, GORM (postgres driver), pgx stdlib, viper, godotenv, gin-gonic/contrib middlewares, bcrypt, testify.
- Implement config loader (env + defaults) returning struct with DB + server settings.
- Provide `.env.example`.

3) Database
- DB connection helper using pgx driver (`dsn` from config).
- Configure connection pool settings.
- Auto-migrate Note model.

4) Domain Model
- Define `Note` model with ID (UUID), Title, Content, optional Tags, `CreatedAt/UpdatedAt/DeletedAt`.
- DTOs: CreateNoteRequest, UpdateNoteRequest, NoteResponse.

5) Repository Layer
- Interface + GORM implementation: Create, GetByID, List (with pagination), Update, Delete (soft delete), Health check (ping).
- Return typed errors (e.g., ErrNotFound).

6) Service Layer
- Validation/business rules: trim, max lengths, required title, handle soft delete.
- Orchestrate transactions if needed.
- Map repo errors to service errors.

7) Controllers & Routing
- Gin handlers using services; respond JSON consistently.
- Routes: `GET /healthz`, `GET /api/v1/notes`, `POST /api/v1/notes`, `GET /api/v1/notes/:id`, `PUT /api/v1/notes/:id`, `DELETE /api/v1/notes/:id`.
- Middlewares: CORS, logging, recovery.

8) Testing
- Service unit tests with fake repo.
- Handler integration tests with httptest + in-memory/test DB.
- Table-driven tests for CRUD.

9) Observability & DX
- Structured logging (fmt or log/slog for now).
- Graceful shutdown with context cancel.
- Add Makefile targets (run/test/lint).

10) Dockerization
- Multi-stage Dockerfile for Go binary.
- Ensure env vars passed; healthcheck on `/healthz`. DONE via compose.
