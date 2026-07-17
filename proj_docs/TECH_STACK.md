# Ruse Technical Stack

| Area | Choice | Purpose |
| --- | --- | --- |
| Frontend | Next.js + TypeScript | Room UI and browser workflows |
| Styling | Tailwind CSS | Interface styling |
| API | Go standard library `net/http` | HTTP API and routing |
| Ephemeral state | Redis + go-redis | Rooms, codes, join requests, capacity |
| Persistent operations | PostgreSQL | Future room-session and operational records |
| Realtime events | Go WebSocket service | Presence, room-state, and chat events |
| Media | Self-hosted LiveKit | WebRTC screen-share and audio routing |
| Deployment | Docker Compose | Initial VPS deployment |
| Reverse proxy | Caddy or Nginx | TLS and HTTPS/WSS termination |

## Current backend dependencies

- `github.com/redis/go-redis/v9` for Redis access.
- `github.com/joho/godotenv` for local `.env` loading.

## Local services

- Go API: `http://localhost:8080`
- Redis: configured through `server/.env` using `REDIS_URL`
- Bruno collection: open the `server` folder and choose the `Local` environment.
