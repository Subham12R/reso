# Reso frontend flow design

## Goal

Build a responsive Reso experience from landing page through authenticated room entry, using the supplied Figma layouts as structural reference and the supplied dark surface system as visual direction.

The implementation must use the existing Instrument Sans and Instrument Serif files, remain dependency-light, and connect to the Go APIs that exist today.

## Scope

### Landing

- Dark, continuous canvas with restrained surface elevation and hairline borders.
- Instrument Serif for editorial display copy and Instrument Sans for interface text.
- A clear primary action that opens the create/join panel.
- Small entrance, focus, and state-transition animations with `prefers-reduced-motion` support.
- Responsive layout from mobile through desktop.

### Create and join

- Create form accepts a display name and calls `POST /api/v1/rooms`.
- Join form accepts a room code and display name and calls `POST /api/v1/rooms/join-requests`.
- Loading, validation, failure, and success states remain inline and keyboard accessible.
- API traffic uses same-origin `/api` paths. Next.js rewrites them to the local Go service during development; production can route the same prefix at the reverse proxy.

### Owner room

- Show the private room code with a copy action.
- Poll `GET /api/v1/rooms/{roomId}/join-requests` for pending requests.
- Allow the owner to approve or reject requests.
- Show the room state and expiry.
- Allow the owner to end the room.
- Use the Figma stage/sidebar composition, without pretending unavailable streaming or chat features work.

### Guest entry

- After requesting access, show a waiting state and poll a claim endpoint.
- Enter the room shell only after approval and successful guest-cookie issuance.
- Show clear rejected, expired, and unavailable states.

### Room shell

- Use the Figma room layout: large stage, compact right rail, and restrained controls.
- If the media-token endpoint is unavailable, show an honest inactive-media state.
- Do not add LiveKit, WebSocket chat, pinning, or participant presence before their backend phases exist.

## Backend contract correction

The current approval handler sets `reso_guest_session` on the owner's response, so the guest cannot complete entry. Correct the flow with the smallest secure handoff:

1. Creating a join request returns a short-lived claim token with the request ID.
2. Only a hash of that token is stored with the join request.
3. Owner approval changes request status but does not set a guest cookie.
4. The guest polls `POST /api/v1/rooms/join-requests/{requestId}/claim` with the claim token in the request body.
5. The token may be reused only while polling that request. Pending returns `202`; rejected or expired returns an appropriate non-success response.
6. An approved claim returns the room ID, sets the HttpOnly guest session cookie in the guest's browser, and invalidates the claim token.
7. A successful claim cannot mint unrelated room access.

Existing owner authorization and room policies remain unchanged.

## Frontend structure

Keep the implementation small:

- `page.tsx` owns the top-level landing and flow selection.
- Focused client components handle the entry panel, owner room, and guest waiting/room states.
- One small API module centralizes JSON requests and error normalization.
- CSS variables in `globals.css` define the dark surface ladder, typography, borders, and motion values.

Do not introduce a state library, animation dependency, component library, or speculative design-system abstraction.

## Accessibility and motion

- Semantic headings, forms, labels, buttons, and status messages.
- Visible focus treatment and sufficient contrast.
- Touch targets of at least 36px.
- CSS transitions and keyframes only.
- Disable nonessential motion under `prefers-reduced-motion: reduce`.

## Error handling

- Preserve server error meaning without exposing sensitive details.
- Keep user-entered values after recoverable failures.
- Treat network failures as retryable.
- Stop polling when the component unmounts or a terminal state is reached.
- Never place session or claim secrets in URLs.

## Verification

- Add or update Go handler/service tests before implementing the guest claim contract.
- Do not add a frontend test framework solely for this pass.
- Run Go tests for affected packages.
- Run frontend lint and production build.
- Verify create, join, approval, rejection, end-room, responsive layout, keyboard navigation, and reduced-motion behavior in a browser against a running local stack where available.

## Explicit exclusions

- LiveKit client integration.
- WebSocket realtime events.
- Chat, participant presence, pinning, fullscreen media, and PWA work.
- External hero imagery or new font downloads.
- New third-party runtime dependencies.
