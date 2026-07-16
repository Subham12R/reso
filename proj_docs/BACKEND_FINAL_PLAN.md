# Final Server Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax for tracking.

**Goal:** Complete the server-side V1 capacity queue, hardening, realtime state, and LiveKit lifecycle.

**Architecture:** Redis remains authoritative for ephemeral room, queue, reservation, presence, and stream-host state. Go exposes HTTP and WebSocket APIs; LiveKit handles media transport and is controlled only through short-lived server-issued tokens and room-service calls.

**Tech Stack:** Go `net/http`, Redis/go-redis, Gorilla WebSocket, LiveKit protocol/server APIs, Bruno.

## Global Constraints

- Server-side only; do not modify `frontend/`.
- Maximum three active rooms globally, enforced atomically with valid reservations.
- Queue reservations last five minutes; disconnected queue sessions expire after 60 seconds.
- Rooms last at most eight hours and allow at most eight participants.
- One screen-share host per room; viewers receive subscribe-only LiveKit grants.
- Room codes and session tokens remain hashed at rest and never enter URLs or logs.
- No recording and no permanent chat storage.
- Preserve existing public API behavior and keep `go test ./...` passing.

---

### Task 1: Atomic capacity queue and reservation claim

**Files:**
- Modify: `server/internal/queue/service.go`
- Modify: `server/internal/rooms/store.go`
- Modify: `server/internal/rooms/redis_store.go`
- Modify: `server/internal/rooms/memory_store.go`
- Modify: `server/internal/rooms/service.go`
- Modify: `server/internal/api/handlers/queue.go`
- Modify: `server/internal/api/handlers/rooms.go`
- Modify: `server/internal/api/router.go`
- Modify: `server/rooms/*.bru`
- Test: `server/tests/queue/queue_test.go`

**Produces:** `POST /api/v1/queue/{queueSessionId}/claim`, atomic reservation-aware room creation, and automatic FIFO promotion after room end.

- [ ] Implement Redis Lua operations that prune expired reservations, count active rooms plus reservations, claim only the caller's valid reservation, and create the room atomically.
- [ ] Make room ending release capacity and promote the oldest live queue session for five minutes.
- [ ] Expire queue sessions missing heartbeats for 60 seconds and skip stale entries during promotion.
- [ ] Add Bruno join, status, heartbeat, leave, and claim requests.
- [ ] Run queue and full Go tests.

### Task 2: HTTP hardening and operational lifecycle

**Files:**
- Create: `server/internal/api/middleware.go`
- Modify: `server/internal/api/router.go`
- Modify: `server/internal/api/handlers/health.go`
- Modify: `server/internal/config/config.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/tests/api/middleware_test.go`

**Produces:** request IDs, JSON errors, security headers, bounded request bodies, rate limiting, readiness, structured logs, and graceful shutdown.

- [ ] Add per-IP limits for room creation, room-code attempts, and queue actions using Redis counters with TTL.
- [ ] Add `X-Request-ID`, CSP, `X-Content-Type-Options`, `Referrer-Policy`, and `Permissions-Policy` headers.
- [ ] Enforce display-name and request-body limits at HTTP trust boundaries.
- [ ] Add `/health` liveness and `/ready` checks for Redis and LiveKit.
- [ ] Shut down on SIGINT/SIGTERM with a bounded context and structured `log/slog` output.
- [ ] Run API and full Go tests.

### Task 3: Authenticated realtime room events

**Files:**
- Create: `server/internal/realtime/hub.go`
- Create: `server/internal/api/handlers/realtime.go`
- Modify: `server/internal/api/router.go`
- Modify: `server/internal/api/handlers/rooms.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/tests/realtime/hub_test.go`

**Produces:** `GET /api/v1/rooms/{roomId}/events` WebSocket with versioned envelopes and authenticated room membership.

- [ ] Authenticate owner or approved guest cookies before upgrading and validate Origin.
- [ ] Track connections and presence with Redis TTLs; cap active participants at eight.
- [ ] Broadcast `join.requested`, `join.approved`, `join.rejected`, `participant.joined`, `participant.left`, `room.ended`, and `stream.host.changed` envelopes.
- [ ] Accept presence heartbeats and room-state requests; enforce read/write deadlines and message limits.
- [ ] Run realtime and full Go tests.

### Task 4: LiveKit stream-host and room cleanup

**Files:**
- Modify: `server/internal/rooms/types.go`
- Modify: `server/internal/rooms/service.go`
- Modify: `server/internal/media/tokens.go`
- Modify: `server/internal/api/handlers/media.go`
- Modify: `server/internal/api/handlers/rooms.go`
- Modify: `server/internal/api/router.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/tests/media/tokens_test.go`

**Produces:** owner-controlled stream-host transfer, publish grants only for the active host, and LiveKit room deletion when a Reso room ends.

- [ ] Persist a single stream-host session identity in room state; creator is initial host.
- [ ] Add owner-only host-transfer endpoint and broadcast `stream.host.changed`.
- [ ] Issue 15-minute subscribe tokens to members and screen-share publish grants only to the current host.
- [ ] Delete the corresponding LiveKit room when Reso ends it; treat an already-absent LiveKit room as success.
- [ ] Run media and full Go tests, then exercise the complete Bruno flow.
