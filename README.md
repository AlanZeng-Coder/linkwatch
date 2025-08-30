# Linkwatch — URL Monitoring Service

A tiny HTTP service for registering URLs, periodically checking them, and exposing their status. Built in Go with SQLite persistence.

## Prerequisites
- Go 1.24+
- Dependencies: Run `go mod tidy` (includes github.com/google/uuid and github.com/mattn/go-sqlite3)
- No external DB required (uses local `./linkwatch.db`)

## How to Run
1. Clone the repo: `git clone https://github.com/AlanZeng-Coder/linkwatch`
2. cd linkwatch
3. Run: `go run main.go`
   - Listens on :8080
   - Env vars (optional, defaults):
     - CHECK_INTERVAL=15s
     - MAX_CONCURRENCY=8
     - HTTP_TIMEOUT=5s
     - SHUTDOWN_GRACE=10s

## How to Test
- Unit tests: `go test ./...`
- Manual with curl:
  - POST: `curl -X POST -H "Content-Type: application/json" -d '{"url": "https://example.com"}' http://localhost:8080/v1/targets`
  - List: `curl 'http://localhost:8080/v1/targets?limit=2'`
  - Results: `curl 'http://localhost:8080/v1/targets/<id>/results?limit=5'`
  - Wait 15s for checks.

## Assumptions
- URLs: HTTP/HTTPS only, canonicalized (lowercase, trim /, drop fragments).
- Checks: Every 15s, retry 5xx/network (2x, backoff 200ms).
- Pagination: Cursor-based (created_at, id order).
- Idempotency: Durable via DB.

Docker: See Dockerfile for containerization.