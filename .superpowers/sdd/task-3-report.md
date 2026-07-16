# Task 3 report: authenticated realtime room events

## Scope

Implemented the server-side WebSocket endpoint at `GET /api/v1/rooms/{roomId}/events`, Redis TTL presence admission, realtime room broadcasts, HTTP mutation publication hooks, origin configuration, and focused realtime tests. No client code or permanent event storage was added.

## TDD evidence

### RED 1: missing realtime feature

Command:

```text
go test ./tests/realtime
```

Expected failure observed before production implementation:

```text
no required module provides package github.com/subham12r/reso/internal/realtime
FAIL github.com/subham12r/reso/tests/realtime [setup failed]
```

### GREEN 1: focused behavior

After adding the minimal hub and handler, the focused suite passed. During protocol review, the server implementation was switched from `x/net/websocket` to Gorilla WebSocket because Gorilla exposes the pong handler needed to extend read deadlines explicitly.

### RED 2: routed upgrade regression

Self-review identified that the existing logging response writer did not preserve `http.Hijacker`. A router-level test was added and failed before the fix:

```text
go test ./tests/realtime -run TestRouterPreservesWebSocketUpgrade -count=1
HTTP status 500
upgrade through router: ... bad status
FAIL
```

### GREEN 2: routed upgrade and focused suite

After delegating `Hijack` through the middleware wrapper:

```text
go test ./tests/realtime -run TestRouterPreservesWebSocketUpgrade -count=1
ok github.com/subham12r/reso/tests/realtime

go test -v ./tests/realtime -count=1
PASS
ok github.com/subham12r/reso/tests/realtime
```

The optional live Redis test was skipped because `REDIS_TEST_URL` was not set. The in-memory focused admission test passed; the Redis adapter uses one Lua script to prune expired sessions and atomically enforce the eight-session limit.

## Verification

```text
go test -race ./tests/realtime -count=1
ok github.com/subham12r/reso/tests/realtime

go test ./... -count=1
all packages passed
```

## Security and lifecycle checks

- Owner or approved guest cookies are authorized before upgrade.
- Origins must exactly match `ALLOWED_ORIGINS`; wildcard entries are ignored, and localhost is allowed only when explicitly configured.
- Stable participant IDs are derived from a one-way digest; room/session secrets are not emitted in URLs, logs, or event payloads.
- Only `presence.heartbeat` and `room.state.request` client events are accepted.
- Reads are bounded and deadline-controlled; pong frames extend the read deadline.
- One writer goroutine serializes event and ping frames; bounded client queues disconnect slow consumers.
- HTTP events are published only after successful join-request, approval, rejection, and room-end service calls.
- Presence entries expire after missed heartbeats and are removed on disconnect.
