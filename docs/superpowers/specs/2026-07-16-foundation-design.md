# Reso foundation design

## Scope

Create the first runnable Reso foundation only:

- a static Next.js landing shell;
- local Instrument Sans and Instrument Serif registration;
- a Go HTTP server with `GET /health`.

## Server structure

```text
server/
├── cmd/api/main.go
├── internal/
│   ├── api/
│   │   ├── router.go
│   │   └── handlers/health.go
│   └── services/health.go
└── internal/api/handlers/health_test.go
```

`cmd/api` starts the process. `api` owns HTTP routing. `handlers` translate HTTP into responses. `services` contains the small application operation. The server uses Go's standard `net/http` package; no dependencies are added.

## Frontend structure

The existing Next.js App Router structure remains in place:

```text
frontend/src/app/
├── layout.tsx
├── globals.css
└── page.tsx
```

`layout.tsx` registers the existing local fonts through `next/font/local` and supplies Reso metadata. `globals.css` defines global colour and font tokens. `page.tsx` is a static, accessible Reso landing shell. It does not imply that creating or joining rooms works yet.

## HTTP contract

`GET /health` returns HTTP 200 and:

```json
{"status":"ok"}
```

## Verification

- Go: `go test ./...` from `server/`.
- Frontend: `npm run lint` and `npm run build` from `frontend/`.

## Explicitly excluded

Redis, PostgreSQL, Docker, LiveKit, WebSockets, room APIs, PWA support, and application state remain unimplemented.
