# Review Notes

## Issue: Legacy cleanup touches removed columns

On a fresh database, the new `jobs` schema no longer includes the host-worker token columns, but `CleanupLegacyHostWorker` still runs an `UPDATE` that references `host_token*` unconditionally. After `AutoMigrate` creates the slim table, this query fails with “column does not exist,” generating a warning on every startup and preventing cleanup when the legacy columns were already removed. Guarding the cleanup when columns are absent (or running it before dropping the fields) would avoid the persistent warning and ensure cleanup runs only when the legacy columns exist.

- Location: `backend/internal/db/cleanup.go` (lines 14-24)
