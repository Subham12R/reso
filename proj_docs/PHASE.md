# Delivery Phases

## Phase 1 — Foundation

Status: complete

- Go API module and package structure
- Redis client and `.env` configuration
- Health endpoint
- Bruno collection and local environment template

## Phase 2 — Room lifecycle

Status: in progress

- Room creation and hashed room codes
- Owner session cookie
- Join-request approval and rejection
- Redis-backed active-room capacity
- Owner room ending and room-state endpoint
- Remaining: empty-room cleanup, participant capacity, and reconnect grace

## Phase 3 — Realtime room

Status: not started

- Authenticated WebSocket endpoint
- Redis-backed presence
- Room and participant event broadcasts

## Phase 4 — Media room

Status: not started

- LiveKit deployment and server-issued tokens
- Single stream-host permission and transfer

## Phase 5 — Collaboration and advanced UI

Status: not started

- Chat, moderation, participant controls, pinning, fullscreen

## Phase 6 — Global capacity queue

Status: not started

- Queue, heartbeats, reservations, and capacity promotion

## Phase 7 — Production hardening

Status: not started

- PostgreSQL operational records, rate limiting, observability, PWA, and deployment hardening
