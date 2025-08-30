# Linkwatch Design Choices & Edge Cases

## Key Choices
- Storage: SQLite for durable persistence (file-based, easy local setup; alternative to preferred Postgres for simplicity). Tables ensure unique URLs and idempotency keys.
- Checker: Ticker for intervals, worker pool (semaphore for concurrency cap), per-host mutex (sync.Map) for serialization. HTTP client with timeout/redirect limit. Retries: exponential backoff on 5xx/network.
- Handler: net/http mux for routing. Canonicalize: lowercase, trim trailing / (root no /), drop ports/fragments.
- Shutdown: Signal notify for SIGTERM/INT, wg.Wait for checks, ctx timeout for grace.
- Config: Env vars with defaults for flexibility (Twelve-Factor App inspired).

## Edge Cases
- Concurrent same-host checks: Mutex serializes, global sem caps total.
- Idempotency: Key durable; same key diff URL returns existing.
- Pagination: Cursor (base64 created_at|id) handles time collisions via id > ?.
- Retries: No on 4xx; timeout/network cancelable via ctx.
- Shutdown mid-check: Grace waits; unfinished lost (no partial save).
- Time precision: created_at nano, but DB ms; sleep in tests mitigates.
- Invalid input: 400 on bad scheme; large lists capped in checkAll.