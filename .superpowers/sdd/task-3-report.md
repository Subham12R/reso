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

## Important findings follow-up

### RED 3: application heartbeat expiry

The focused expiry test was written before the watchdog API and failed to build as expected:

```text
go test ./tests/realtime -run 'TestHubExpiresClientWithoutApplicationHeartbeat|TestSlowConsumerEvictionUsesParticipantLifecycle' -count=1
tests/realtime/hub_test.go:70:18: undefined: realtime.NewHubWithPresenceTTL
FAIL
```

Root cause: Redis presence expiry was passive. Protocol pong traffic could keep the socket readable forever while no application heartbeat refreshed Redis. Each joined client now has a hub-owned application-heartbeat timer. Only a successful `presence.heartbeat` resets it; expiry routes through normal lifecycle removal and closes the socket.

### RED 4: slow-consumer lifecycle

The slow-consumer reproduction filled one owner's bounded outbound queue while another participant kept reading. Before the fix, the slow owner closed but the observer never received `participant.left` or `stream.host.changed`.

Root cause: broadcast eviction directly deleted the client and Redis presence instead of using the normal lifecycle path. Join, explicit leave, heartbeat expiry, and slow-consumer eviction now share one removal path, including lifecycle broadcasts and presence cleanup.

### Added focused coverage

- Application-heartbeat expiry closes the client and emits `participant.left` without relying on protocol read deadlines.
- Slow owner eviction emits both `participant.left` and `stream.host.changed`.
- An approved guest cookie can upgrade and receives a participant-role join envelope.
- Successful approval, rejection, and room-end HTTP handlers publish their corresponding events.
- The existing join-request test continues to assert that room/session secrets are absent from event payloads.

### Follow-up verification

```text
REDIS_TEST_URL=redis://127.0.0.1:6379/1 go test -v ./tests/realtime -run TestRedisPresenceAtomicallyLimitsRoom -count=1
PASS
ok github.com/subham12r/reso/tests/realtime

go test -race -v ./tests/realtime -count=1 -timeout=30s
PASS
ok github.com/subham12r/reso/tests/realtime

go test ./... -count=1
all packages passed
```

The first race run exposed a concurrent-map write in the in-memory test presence fake when watchdog cleanup and test cleanup overlapped. Making the fake thread-safe removed the race; no production race was reported.
