# Ruse

Ruse is a private, temporary room for small-group screen sharing. Create a room, share its code, approve people as they arrive, then collaborate over screen share, camera, microphone, and ephemeral chat without accounts, recordings, or persistent chat history.

**Live:** [ruse.monostack.in](https://ruse.monostack.in) | **Health:** [ruseapi.monostack.in/health](https://ruseapi.monostack.in/health)

## What it does

- Creates temporary rooms with random share codes and owner-controlled admission.
- Supports up to eight connected participants per room and three active rooms globally.
- Streams screen video and optional system audio through LiveKit.
- Lets every participant use camera and microphone; the designated stream host controls screen sharing.
- Includes participant tiles, pinning, draggable/resizable pinned video, fullscreen, and ephemeral room chat.
- Cleans up rooms when the owner ends them, when they expire after eight hours, or after they remain empty for a minute.

## Architecture

```text
Browser (Next.js) -- HTTPS/WSS --> Nginx --> Next.js frontend
       |                              `--> Go API --> Redis
       `---------- LiveKit WebRTC ----------> LiveKit Cloud
```

The Go API owns room access, approvals, presence, and LiveKit token issuance. Redis stores temporary room state, hashed codes and session tokens, capacity reservations, and presence. LiveKit carries media and ephemeral data messages.

## Stack

- Next.js 16, React, TypeScript, Tailwind CSS, and `livekit-client`
- Go 1.26, `net/http`, Gorilla WebSocket, and the LiveKit Go SDK
- Redis 7 for temporary state and capacity control
- LiveKit Cloud for WebRTC media
- Docker Compose, GHCR, Nginx, and Let's Encrypt for production

## Local development

Prerequisites: Node.js 22+, Go 1.26+, Redis, and a LiveKit Cloud project.

Start Redis:

```bash
docker run --rm -p 6379:6379 redis:7-alpine
```

Create `server/.env`:

```dotenv
REDIS_URL=redis://127.0.0.1:6379/0
LIVEKIT_URL=wss://your-project.livekit.cloud
LIVEKIT_API_KEY=your-key
LIVEKIT_API_SECRET=your-secret
COOKIE_SECURE=false
TRUST_PROXY_HEADERS=false
```

Run the API and frontend in separate terminals:

```bash
cd server
go run ./cmd/api
```

```bash
cd frontend
npm ci
npm run dev
```

Open `http://localhost:3000`.

## Production deployment

The GitHub Actions workflow publishes API and frontend images to GHCR after changes under `server/` or `frontend/` merge into `main`.

1. Copy `deploy.env.example` to `.env.production` and set real values:

   ```dotenv
   REDIS_PASSWORD=a-long-random-alphanumeric-password
   LIVEKIT_URL=wss://your-project.livekit.cloud
   LIVEKIT_API_KEY=your-key
   LIVEKIT_API_SECRET=your-secret
   TRUST_PROXY_HEADERS=true
   COOKIE_SECURE=true
   ALLOWED_ORIGINS=https://ruse.monostack.in
   ```

2. Start the isolated production stack:

   ```bash
   docker compose -f docker-compose.production.yml --env-file .env.production pull
   docker compose -f docker-compose.production.yml --env-file .env.production up -d
   ```

   The frontend and API bind only to `127.0.0.1:3001` and `127.0.0.1:8091`. Redis has no host port.

3. Issue certificates for both domains, then install the included Nginx configuration:

   ```bash
   sudo certbot certonly --standalone -d ruse.monostack.in -d ruseapi.monostack.in
   sudo install -m 644 deploy/nginx/ruse.conf /etc/nginx/sites-available/ruse
   sudo nginx -t && sudo systemctl reload nginx
   ```

   If Nginx already uses port 80, stop it only while Certbot performs the standalone challenge.

4. Verify:

   ```bash
   curl -I https://ruse.monostack.in
   curl -fsS https://ruseapi.monostack.in/health
   ```

## Security model

- Room codes and browser session tokens are generated randomly and stored only as hashes.
- Session cookies are `HttpOnly`, `Secure` in production, and `SameSite=Lax`.
- Secure-cookie deployments require an explicit `ALLOWED_ORIGINS` list; unsafe requests from other origins are rejected.
- LiveKit access tokens are server-issued, scoped to one room, and expire after 15 minutes.
- Redis is private to the Ruse Compose network and requires a password.
- Room creation, join attempts, queue actions, and room mutations are rate-limited.
- Nginx applies HTTPS redirects, HSTS, `nosniff`, frame protection, referrer policy, permissions policy, and a baseline CSP.

## API highlights

| Endpoint | Purpose |
| --- | --- |
| `POST /api/v1/rooms` | Create a room and owner session |
| `POST /api/v1/rooms/join-requests` | Request entry with a room code |
| `POST /api/v1/rooms/{roomId}/join-requests/{requestId}/approve` | Approve a guest |
| `POST /api/v1/rooms/{roomId}/media/token` | Obtain a short-lived LiveKit token |
| `GET /api/v1/rooms/{roomId}/events` | Authenticated room WebSocket |
| `POST /api/v1/rooms/{roomId}/end` | End a room as its owner |
| `GET /health` | Liveness check |
| `GET /ready` | Redis and LiveKit readiness check |

## Verification

```bash
cd server
go test ./...
go vet ./...

cd ../frontend
npm ci
npm run lint
npm run build
npm audit --omit=dev
```

## Project layout

```text
frontend/       Next.js room UI and LiveKit client
server/         Go API, room lifecycle, Redis state, WebSocket hub
infra/livekit/  Local LiveKit development configuration
deploy/nginx/   Production reverse-proxy configuration
```

## License

Private project. All rights reserved.
