# Reso Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable Reso landing shell and a Go health endpoint using the agreed server boundaries.

**Architecture:** The Next.js App Router frontend remains static and uses the supplied local font files through `next/font/local`. The Go service follows `cmd` for process startup, `internal/api` for HTTP routing, `handlers` for HTTP translation, and `services` for application data. `GET /health` is the sole exposed endpoint.

**Tech Stack:** Next.js 16, React 19, TypeScript, Tailwind CSS 4, Go 1.26, Go standard library `net/http`.

## Global Constraints

- Use the existing local Instrument Sans and Instrument Serif files; do not fetch fonts externally.
- Add no frontend or Go dependencies.
- Do not add Redis, PostgreSQL, Docker, LiveKit, WebSockets, room APIs, or PWA support.
- The health response is HTTP 200 with exactly `{"status":"ok"}`.

---

## File Structure

- Modify: `frontend/src/app/layout.tsx` — local fonts and Reso metadata.
- Modify: `frontend/src/app/globals.css` — global Reso colours and font tokens.
- Modify: `frontend/src/app/page.tsx` — static landing shell.
- Create: `server/internal/api/handlers/health_test.go` — health handler behavior test.
- Create: `server/internal/services/health.go` — health response data.
- Create: `server/internal/api/handlers/health.go` — JSON HTTP response.
- Create: `server/internal/api/router.go` — route registration.
- Create: `server/cmd/api/main.go` — executable HTTP server.

### Task 1: Build the Go health endpoint test-first

**Files:**
- Create: `server/internal/api/handlers/health_test.go`
- Create: `server/internal/services/health.go`
- Create: `server/internal/api/handlers/health.go`
- Create: `server/internal/api/router.go`
- Create: `server/cmd/api/main.go`

**Interfaces:**
- Produces: `services.Health() HealthResponse` where `HealthResponse` has `Status string` with JSON name `status`.
- Produces: `handlers.NewHealthHandler() http.Handler`.
- Produces: `api.NewRouter() http.Handler`, registering `GET /health`.

- [ ] **Step 1: Write the failing test**

```go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/subham12r/reso/internal/api/handlers"
)

func TestHealthHandlerReturnsOK(t *testing.T) {
    request := httptest.NewRequest(http.MethodGet, "/health", nil)
    recorder := httptest.NewRecorder()

    handlers.NewHealthHandler().ServeHTTP(recorder, request)

    if recorder.Code != http.StatusOK {
        t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
    }
    if got := recorder.Body.String(); got != "{\"status\":\"ok\"}\\n" {
        t.Fatalf("body = %q, want health JSON", got)
    }
}
```

- [ ] **Step 2: Run the test and verify it fails because the handler does not exist**

Run: `go test ./internal/api/handlers`

Expected: compile failure mentioning `handlers.NewHealthHandler` is undefined.

- [ ] **Step 3: Add the minimal health service and handler**

```go
// server/internal/services/health.go
package services

type HealthResponse struct {
    Status string `json:"status"`
}

func Health() HealthResponse {
    return HealthResponse{Status: "ok"}
}
```

```go
// server/internal/api/handlers/health.go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/subham12r/reso/internal/services"
)

func NewHealthHandler() http.Handler {
    return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        writer.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(writer).Encode(services.Health())
    })
}
```

- [ ] **Step 4: Add the router and executable entrypoint**

```go
// server/internal/api/router.go
package api

import (
    "net/http"

    "github.com/subham12r/reso/internal/api/handlers"
)

func NewRouter() http.Handler {
    router := http.NewServeMux()
    router.Handle("GET /health", handlers.NewHealthHandler())
    return router
}
```

```go
// server/cmd/api/main.go
package main

import (
    "log"
    "net/http"

    "github.com/subham12r/reso/internal/api"
)

func main() {
    server := &http.Server{Addr: ":8080", Handler: api.NewRouter()}
    log.Fatal(server.ListenAndServe())
}
```

- [ ] **Step 5: Run the focused test and all Go tests**

Run: `go test ./internal/api/handlers; go test ./...`

Expected: both commands exit 0.

### Task 2: Replace the generated frontend with the Reso shell

**Files:**
- Modify: `frontend/src/app/layout.tsx`
- Modify: `frontend/src/app/globals.css`
- Modify: `frontend/src/app/page.tsx`

**Interfaces:**
- Consumes: `frontend/public/fonts/InstrumentSans-VariableFont_wdth,wght.ttf` and `frontend/public/fonts/InstrumentSerif-Regular.ttf`.
- Produces: root route `/` with static Reso copy and no client-side behavior.

- [ ] **Step 1: Register local fonts and metadata in `frontend/src/app/layout.tsx`**

```tsx
import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";

const instrumentSans = localFont({
  src: "../../public/fonts/InstrumentSans-VariableFont_wdth,wght.ttf",
  variable: "--font-instrument-sans",
});

const instrumentSerif = localFont({
  src: "../../public/fonts/InstrumentSerif-Regular.ttf",
  variable: "--font-instrument-serif",
  weight: "400",
});

export const metadata: Metadata = {
  title: "Reso | Private watch rooms",
  description: "Temporary private rooms for sharing a screen and watching together.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${instrumentSans.variable} ${instrumentSerif.variable}`}>
      <body>{children}</body>
    </html>
  );
}
```

- [ ] **Step 2: Set the minimal global tokens in `frontend/src/app/globals.css`**

```css
@import "tailwindcss";

:root {
  --background: #111111;
  --foreground: #f4f4f0;
  --muted: #a4a4a0;
  --line: #2b2b2a;
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --font-sans: var(--font-instrument-sans);
  --font-serif: var(--font-instrument-serif);
}

* { box-sizing: border-box; }

body {
  margin: 0;
  background: var(--background);
  color: var(--foreground);
  font-family: var(--font-instrument-sans), sans-serif;
}
```

- [ ] **Step 3: Replace `frontend/src/app/page.tsx` with a static landing shell**

```tsx
export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-6xl flex-col px-6 py-8 sm:px-10">
      <header className="flex items-center justify-between border-b border-[var(--line)] pb-5">
        <span className="font-serif text-3xl">reso</span>
        <span className="text-sm text-[var(--muted)]">Private watch rooms</span>
      </header>
      <section className="flex flex-1 flex-col justify-center py-20">
        <p className="mb-5 text-sm text-[var(--muted)]">Temporary. Private. No accounts.</p>
        <h1 className="max-w-3xl font-serif text-5xl leading-none sm:text-7xl">Watch together without making a place to stay.</h1>
        <p className="mt-8 max-w-xl text-lg leading-8 text-[var(--muted)]">Reso is a focused room for sharing a screen, talking, and leaving no permanent room behind.</p>
      </section>
      <footer className="border-t border-[var(--line)] pt-5 text-sm text-[var(--muted)]">Foundation build — room creation is coming next.</footer>
    </main>
  );
}
```

- [ ] **Step 4: Run frontend verification**

Run: `npm run lint; npm run build`

Expected: both commands exit 0.
