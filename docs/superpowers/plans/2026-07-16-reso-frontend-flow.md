# Reso Frontend Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a responsive landing-to-room flow backed by the existing Go room APIs, including a secure guest approval handoff.

**Architecture:** Keep the page server-rendered at the outer boundary and place interactive flow state in one client component. Use same-origin `/api` requests through a Next.js rewrite, focused form and room components, and a small typed request module. Extend the Go room service with a short-lived claim-token exchange so the approved guest receives their own HttpOnly cookie.

**Tech Stack:** Go 1.25 `net/http`, Next.js 16 App Router, React 19, TypeScript 5, Tailwind CSS 4, local Instrument Sans and Instrument Serif fonts.

## Global Constraints

- Use the supplied Figma layouts as structural reference and the supplied dark surface system as visual direction.
- Use only the existing Instrument Sans and Instrument Serif font files.
- Do not add third-party runtime dependencies, a state library, an animation library, or a frontend test framework.
- Keep secrets out of URLs and preserve HttpOnly cookies for authenticated room sessions.
- Respect `prefers-reduced-motion` and keep all controls keyboard accessible.
- Do not implement LiveKit playback, WebSockets, chat, participant presence, pinning, fullscreen media, or PWA behavior.
- Preserve the pre-existing edits in `server/internal/api/middleware.go` and `server/tests/api/hardening_test.go`.

---

### Task 1: Secure guest claim contract

**Files:**
- Modify: `server/internal/rooms/types.go`
- Modify: `server/internal/rooms/service.go`
- Modify: `server/internal/api/handlers/rooms.go`
- Modify: `server/internal/api/router.go`
- Test: `server/tests/rooms/service_test.go`
- Test: `server/tests/api/handlers/rooms_test.go`

**Interfaces:**
- Produces: `CreatedJoinRequest{Request JoinRequest, ClaimToken string}`.
- Produces: `JoinClaim{Status JoinRequestStatus, RoomID string, SessionToken string}`.
- Produces: `RoomService.ApproveJoinRequest(...) (JoinRequest, error)`; approval changes status only.
- Produces: `RoomService.ClaimJoinRequest(requestID, claimToken string) (JoinClaim, error)`.
- Produces: `POST /api/v1/rooms/join-requests/{requestId}/claim` with `{ "claimToken": string }`.

- [ ] **Step 1: Write failing service tests for claim lifecycle**

Add tests that create a room and join request, assert the claim token is non-empty but not stored in plaintext, assert a pending claim returns `pending`, approve it, assert the approved claim returns the room ID and session token, assert `AuthorizeRoomSession` accepts that token, and assert a second claim fails.

```go
func TestClaimJoinRequestIssuesGuestSessionAfterApproval(t *testing.T) {
    service := rooms.NewRoomService()
    created, _ := service.CreateRoom("Owner")
    requested, err := service.CreateJoinRequest(created.Code, "Guest")
    if err != nil || requested.ClaimToken == "" {
        t.Fatalf("CreateJoinRequest() = %#v, %v", requested, err)
    }
    pending, err := service.ClaimJoinRequest(requested.Request.ID, requested.ClaimToken)
    if err != nil || pending.Status != rooms.JoinRequestPending {
        t.Fatalf("pending claim = %#v, %v", pending, err)
    }
    if _, err = service.ApproveJoinRequest(created.Room.ID, requested.Request.ID, created.OwnerSessionToken); err != nil {
        t.Fatal(err)
    }
    approved, err := service.ClaimJoinRequest(requested.Request.ID, requested.ClaimToken)
    if err != nil || approved.RoomID != created.Room.ID || approved.SessionToken == "" {
        t.Fatalf("approved claim = %#v, %v", approved, err)
    }
    if role, err := service.AuthorizeRoomSession(created.Room.ID, approved.SessionToken); err != nil || role != rooms.SessionRoleParticipant {
        t.Fatalf("AuthorizeRoomSession() = %q, %v", role, err)
    }
    if _, err := service.ClaimJoinRequest(requested.Request.ID, requested.ClaimToken); err == nil {
        t.Fatal("second claim unexpectedly succeeded")
    }
}
```

- [ ] **Step 2: Run the focused service test and verify RED**

Run: `go test ./tests/rooms -run TestClaimJoinRequestIssuesGuestSessionAfterApproval -v`

Expected: compilation fails because `CreatedJoinRequest`, `ClaimToken`, and `ClaimJoinRequest` do not exist.

- [ ] **Step 3: Implement the minimum service contract**

Add `ClaimTokenHash` to `JoinRequest`, return a `CreatedJoinRequest` from creation, change owner approval to return the updated `JoinRequest` without generating a guest token, and exchange the submitted claim token only after approval. Update every `CreateJoinRequest` caller to read `.Request`.

```go
type CreatedJoinRequest struct {
    Request    JoinRequest
    ClaimToken string
}

type JoinClaim struct {
    Status       JoinRequestStatus
    RoomID       string
    SessionToken string
}

func (service *RoomService) ClaimJoinRequest(requestID, claimToken string) (JoinClaim, error) {
    ctx := context.Background()
    request, err := service.store.FindJoinRequest(ctx, requestID)
    if err != nil || request.ClaimTokenHash == "" || hashSecret(claimToken) != request.ClaimTokenHash {
        return JoinClaim{}, ErrUnauthorized
    }
    if time.Now().After(request.ExpiresAt) {
        return JoinClaim{}, ErrJoinRequestExpired
    }
    claim := JoinClaim{Status: request.Status, RoomID: request.RoomID}
    if request.Status != JoinRequestApproved {
        return claim, nil
    }
    request.GuestSessionHash = request.ClaimTokenHash
    request.ClaimTokenHash = ""
    if err := service.store.UpdateJoinRequest(ctx, request); err != nil {
        return JoinClaim{}, err
    }
    claim.SessionToken = claimToken
    return claim, nil
}
```

- [ ] **Step 4: Run room service tests and verify GREEN**

Run: `go test ./tests/rooms -v`

Expected: all room service tests pass.

- [ ] **Step 5: Write failing handler tests for guest-owned cookie issuance**

Update existing join-request call sites to use `.Request`. Add handler tests asserting create returns `claimToken`, owner approval returns no guest cookie, pending claim returns `202`, and approved claim returns `200`, `roomId`, and `reso_guest_session`.

```go
claimBody := []byte(`{"claimToken":"` + requested.ClaimToken + `"}`)
request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/join-requests/"+requested.Request.ID+"/claim", bytes.NewReader(claimBody))
request.SetPathValue("requestId", requested.Request.ID)
recorder := httptest.NewRecorder()
handlers.NewClaimJoinRequestHandler(service).ServeHTTP(recorder, request)
if recorder.Code != http.StatusOK {
    t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
}
if cookies := recorder.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "reso_guest_session" {
    t.Fatalf("cookies = %#v", cookies)
}
```

- [ ] **Step 6: Run the focused handler tests and verify RED**

Run: `go test ./tests/api/handlers -run 'Test(CreateJoinRequest|ApproveJoinRequest|ClaimJoinRequest)' -v`

Expected: compilation fails because the response and handler do not yet expose claim behavior.

- [ ] **Step 7: Implement handler and route**

Return `claimToken` from join creation. Make owner approval status-only. Add `NewClaimJoinRequestHandler`, mapping pending to `202`, rejected to `403`, expired to `410`, invalid claims to `404`, and approved to `200` plus the secure guest cookie. Register:

```go
router.Handle("POST /api/v1/rooms/join-requests/{requestId}/claim", handlers.NewClaimJoinRequestHandler(roomService))
```

- [ ] **Step 8: Run backend verification**

Run: `go test ./...`

Expected: all Go tests pass.

- [ ] **Step 9: Commit the backend contract**

```bash
git add server/internal/rooms/types.go server/internal/rooms/service.go server/internal/api/handlers/rooms.go server/internal/api/router.go server/tests/rooms/service_test.go server/tests/api/handlers/rooms_test.go
git commit -m "feat: complete guest room claim flow"
```

---

### Task 2: Same-origin typed frontend API

**Files:**
- Modify: `frontend/next.config.ts`
- Create: `frontend/src/app/lib/api.ts`

**Interfaces:**
- Consumes: Task 1 HTTP routes.
- Produces: `createRoom`, `requestJoin`, `claimJoin`, `listPendingRequests`, `approveJoin`, `rejectJoin`, `getRoomState`, `endRoom`, and `getMediaAccess`.

- [ ] **Step 1: Add the local API rewrite**

Use the documented Next.js 16 `rewrites()` API and a server-only environment variable with a safe local default.

```ts
import type { NextConfig } from "next";

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
};

export default nextConfig;
```

- [ ] **Step 2: Add the typed request module**

Implement one `request<T>` wrapper using `credentials: "include"`, JSON bodies, and a short `ApiError`. Export exact response types and endpoint helpers. The claim helper must return a discriminated union:

```ts
export type ClaimResult =
  | { status: "pending" | "rejected" }
  | { status: "approved"; roomId: string };

export async function claimJoin(requestId: string, claimToken: string) {
  return request<ClaimResult>(`/api/v1/rooms/join-requests/${requestId}/claim`, {
    method: "POST",
    body: JSON.stringify({ claimToken }),
  }, [202, 403]);
}
```

`request` must treat listed response statuses as parseable application states and throw `ApiError` for all other non-2xx responses.

- [ ] **Step 3: Verify TypeScript and lint**

Run: `npm run lint`

Expected: zero ESLint errors.

- [ ] **Step 4: Commit the API boundary**

```bash
git add frontend/next.config.ts frontend/src/app/lib/api.ts
git commit -m "feat: add typed room api client"
```

---

### Task 3: Landing and entry flow

**Files:**
- Modify: `frontend/src/app/page.tsx`
- Create: `frontend/src/app/components/reso-app.tsx`
- Create: `frontend/src/app/components/entry-panel.tsx`

**Interfaces:**
- Consumes: `createRoom()` and `requestJoin()` from Task 2.
- Produces: transitions to owner, guest-waiting, and landing states.

- [ ] **Step 1: Keep the route server-rendered and isolate interactivity**

Replace `page.tsx` with:

```tsx
import { ResoApp } from "./components/reso-app";

export default function Home() {
  return <ResoApp />;
}
```

- [ ] **Step 2: Implement the flow state machine**

Create a client component with this explicit state union and no state library:

```ts
type View =
  | { kind: "landing" }
  | { kind: "entry"; mode: "create" | "join" }
  | { kind: "owner"; roomId: string; code: string }
  | { kind: "waiting"; requestId: string; claimToken: string }
  | { kind: "guest"; roomId: string };
```

Render the Figma-inspired landing hero, a compact command-palette preview, three capability cards, and the footer. Use one white primary CTA per fold. Pass callbacks to `EntryPanel`; do not place network code in the visual landing markup.

- [ ] **Step 3: Implement accessible create/join forms**

`EntryPanel` must use labeled inputs, native form submission, `aria-live="polite"` status copy, a 50-character display-name limit, and a normalized room code. Disable only the submitting form. On success call:

```ts
onCreated({ kind: "owner", roomId: result.roomId, code: result.code });
onRequested({ kind: "waiting", requestId: result.requestId, claimToken: result.claimToken });
```

- [ ] **Step 4: Run frontend checks**

Run: `npm run lint`

Expected: zero ESLint errors.

Run: `npm run build`

Expected: successful production build with `/` prerendered and no hydration errors.

- [ ] **Step 5: Commit landing and entry**

```bash
git add frontend/src/app/page.tsx frontend/src/app/components/reso-app.tsx frontend/src/app/components/entry-panel.tsx
git commit -m "feat: build landing and room entry flow"
```

---

### Task 4: Owner, waiting, and room states

**Files:**
- Create: `frontend/src/app/components/room-shell.tsx`
- Modify: `frontend/src/app/components/reso-app.tsx`

**Interfaces:**
- Consumes: all remaining Task 2 API helpers.
- Produces: owner approval/rejection/end actions, guest claim polling, room state display, and unavailable-media feedback.

- [ ] **Step 1: Implement guest claim polling**

In `ResoApp`, poll every two seconds with an abort flag. Stop on unmount, approval, rejection, or error. Never persist `claimToken` or put it in a URL.

```tsx
useEffect(() => {
  if (view.kind !== "waiting") return;
  let active = true;
  const poll = async () => {
    const result = await claimJoin(view.requestId, view.claimToken);
    if (!active) return;
    if (result.status === "approved") setView({ kind: "guest", roomId: result.roomId });
    else if (result.status === "pending") window.setTimeout(poll, 2000);
    else setWaitingError("The owner declined this request.");
  };
  void poll();
  return () => { active = false; };
}, [view]);
```

- [ ] **Step 2: Implement the Figma-inspired room shell**

Use a two-column desktop layout that collapses to one column. The stage contains an inactive-media message and disabled-looking share affordance. The rail contains room status, expiry, and role-specific content. Owner mode shows code, copy action, pending requests, approve/reject buttons, and end-room. Guest mode shows approved status and room state.

- [ ] **Step 3: Add owner polling and terminal cleanup**

Poll room state every ten seconds for both roles and pending join requests every three seconds for owners. After approve/reject, refresh pending requests immediately. After end-room, stop polling and render the ended state.

- [ ] **Step 4: Handle media readiness honestly**

Call `getMediaAccess(roomId)` once on entry. On `503`, render “Screen sharing is not configured on this server yet.” Do not install LiveKit or fake playback controls.

- [ ] **Step 5: Run frontend checks**

Run: `npm run lint`

Expected: zero ESLint errors.

Run: `npm run build`

Expected: successful production build.

- [ ] **Step 6: Commit room states**

```bash
git add frontend/src/app/components/reso-app.tsx frontend/src/app/components/room-shell.tsx
git commit -m "feat: add authenticated room shell"
```

---

### Task 5: Visual system, micro-animation, and end-to-end verification

**Files:**
- Modify: `frontend/src/app/globals.css`
- Modify: `frontend/src/app/layout.tsx`

**Interfaces:**
- Consumes: semantic class names from Tasks 3 and 4.
- Produces: final responsive, accessible presentation.

- [ ] **Step 1: Apply the dark surface and typography tokens**

Define the canvas `#07080a`, surface ladder through `#121212`, hairline `#242728`, off-white ink, muted body colors, compact radii, and site-wide font feature settings. Keep Instrument Serif limited to display copy and Instrument Sans for interface text.

```css
:root {
  --canvas: #07080a;
  --surface: #0d0d0d;
  --surface-elevated: #101111;
  --surface-card: #121212;
  --hairline: #242728;
  --ink: #f4f4f6;
  --body: #cdcdcd;
  --mute: #9c9c9d;
}

body {
  background: var(--canvas);
  color: var(--ink);
  font-family: var(--font-instrument-sans), sans-serif;
  font-feature-settings: "calt", "kern", "liga";
}
```

- [ ] **Step 2: Add restrained CSS-only motion**

Add a single hero entrance, card lift/fade, button press, panel transition, and soft active-row movement. No looping decorative animation. Disable animation and smooth scrolling for reduced motion.

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}
```

- [ ] **Step 3: Update metadata copy**

Keep the title `Reso | Private watch rooms` and change the description only if the final landing copy needs exact alignment. Do not change font loading.

- [ ] **Step 4: Run all automated verification**

Run: `go test ./...` from `server`.

Expected: all Go tests pass.

Run: `npm run lint` from `frontend`.

Expected: zero ESLint errors.

Run: `npm run build` from `frontend`.

Expected: successful production build.

- [ ] **Step 5: Run browser verification**

Start the Go API and Next dev server. Verify at 1440px, 768px, and 390px widths:

- landing hierarchy and one primary white CTA per fold;
- keyboard focus and form validation;
- owner create flow and code copy;
- guest request, pending, approval, rejection, and room entry;
- owner pending-request refresh and end-room state;
- honest media-unavailable state;
- reduced-motion emulation removes nonessential motion.

- [ ] **Step 6: Final diff audit and commit**

Run: `git diff --check`

Expected: no whitespace errors.

Confirm the two pre-existing hardening files retain their original user changes and are not included in feature commits.

```bash
git add frontend/src/app/globals.css frontend/src/app/layout.tsx
git commit -m "style: polish reso room experience"
```
