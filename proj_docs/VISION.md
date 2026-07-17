# Ruse — Product Requirements & Technical Design

**Document type:** Product Requirements Document (PRD) + high-level technical design  
**Product:** Ruse
**Version:** 1.0  
**Status:** Approved direction for V1 planning  
**Primary platform:** Chromium-based browsers and installable PWA  
**Target deployment:** Self-hosted on a 4 vCPU / 8 GB VPS  

---

## 1. Product summary

Ruse is a private, temporary watch-room platform that lets a small group of friends share a desktop or browser tab, stream its audio and video in real time, talk through room chat, and watch together without creating accounts.

Each room is protected by a short-lived private room code. Rooms are ephemeral: when the session ends, the room code becomes invalid and the associated temporary state is deleted. The platform will support a maximum of three active rooms globally in V1, with up to eight participants per room.

Ruse is designed for content that the host is authorized to display or share. It does not provide a content catalogue, content hosting, downloading, recording, DRM circumvention, or rebroadcasting features.

---

## 2. Product vision

Create the cleanest possible private watch-room experience for small groups:

1. Open Ruse.
2. Create a temporary room.
3. Share the room code privately.
4. Select a browser tab, application window, or desktop surface.
5. Include system or tab audio when the browser exposes it.
6. Watch and chat together.
7. End the room and permanently invalidate the code.

The product should feel closer to a focused media-room utility than a social network or conferencing suite.

---

## 3. Core principles

### 3.1 Private by default

- No public room discovery.
- No searchable room directory.
- No room can be joined without its current room code.
- Room codes expire permanently when the room ends.
- No account, profile, follower, or social graph system.

### 3.2 Ephemeral by design

- Active room state lives primarily in Redis.
- Chat history is not permanently stored in V1.
- Participant identity is session-based.
- Room state is destroyed after termination.
- PostgreSQL stores only operational and aggregate records where required.

### 3.3 Minimal interface

- Dark neutral surfaces.
- No decorative gradients.
- No glassmorphism-heavy treatment.
- No unnecessary animation.
- Clear hierarchy between stream, participants, controls, and chat.
- Desktop layout must closely follow the supplied reference.

### 3.4 Predictable capacity

- Maximum active rooms globally: **3**.
- Maximum participants per room: **8**.
- Maximum active screen shares per room: **1**.
- Queue additional creators when all room slots are occupied.

### 3.5 Host-controlled media

- One active host stream per room.
- Stream ownership can be transferred to another participant.
- Participants may mute the incoming stream audio locally.
- Participants may mute their own microphone locally and at publication level.

---

## 4. Goals

### 4.1 V1 goals

- Create and join private temporary rooms without login.
- Stream a selected screen, window, or browser tab.
- Capture and transmit shared-surface audio when available.
- Support up to eight participants per room.
- Support up to three rooms globally.
- Provide real-time room chat.
- Allow every participant to control local stream volume and mute state.
- Allow the active stream to be pinned as a floating always-visible player inside the app.
- Support installation as a PWA.
- Maintain room state through temporary network interruptions.
- Queue room creators when all room slots are active.
- Destroy room access and temporary data after room termination.

### 4.2 Non-goals for V1

- User accounts or authentication profiles.
- Public rooms or room discovery.
- Movie or media catalogue.
- Server-side recording.
- Video-on-demand hosting.
- File uploads for copyrighted films.
- DRM bypassing or capture workarounds.
- Multiple simultaneous screen shares.
- More than eight participants in a room.
- Native iOS or Android application.
- Safari- or Firefox-first compatibility.
- Permanent chat history.
- Email or push notifications for the creation queue.
- Payments or subscriptions.

---

## 5. Target users

### Primary user

A small group of friends who want to watch authorized content together in a temporary private room.

### Secondary user

A remote team or study group that wants to share a screen and discuss it without creating accounts or scheduling a permanent meeting.

### User expectations

- Room creation should take seconds.
- Joining should require only a code and display name.
- Stream controls should be obvious.
- The interface should not resemble a complex enterprise meeting platform.
- Leaving or ending a room should clearly explain what happens to the code.

---

## 6. Platform and compatibility scope

### V1 supported environment

- Latest stable Google Chrome.
- Latest stable Microsoft Edge.
- Latest stable Brave.
- Installed PWA based on a supported Chromium browser.
- Desktop operating systems as the primary environment.

### Compatibility policy

Ruse will use capability detection rather than assuming every capture option is available. The product must clearly communicate:

- Whether the selected share source includes audio.
- Whether the browser exposed desktop or tab audio.
- Whether the user denied capture permission.
- Whether a selected source can only provide video.

Ruse must never imply that desktop audio can be captured silently or without explicit browser permission.

---

## 7. User roles

### 7.1 Room creator

The participant who successfully creates the temporary room.

Permissions:

- Copy and share the room code.
- Start the initial screen share.
- End the room for everyone.
- Transfer host/streaming permission.
- Remove disruptive participants.
- Lock or unlock further joins.

### 7.2 Active stream host

The participant currently allowed to publish the primary screen-share track.

Permissions:

- Start or stop screen sharing.
- Choose a share source.
- Include shared-surface audio when available.
- Transfer stream ownership if allowed by the creator.

The creator and stream host may be the same or different participants.

### 7.3 Participant

Permissions:

- Join with a valid room code.
- Watch the active shared stream.
- Send and receive room chat messages.
- Mute or unmute incoming stream audio locally.
- Control local stream volume.
- Publish or mute their own microphone if room voice is enabled.
- Pin or unpin the active stream as an in-app floating player.
- Leave the room.

---

## 8. Primary user journeys

## 8.1 Create a room

1. User opens Ruse.
2. User selects **Create private room**.
3. Ruse checks the global room capacity atomically.
4. If fewer than three room slots are occupied, a temporary room reservation is created.
5. User enters a display name.
6. Backend creates the application room and corresponding LiveKit room.
7. User receives a short private room code.
8. User enters the room as creator.
9. User may start screen sharing immediately.

### Failure state: capacity full

1. User attempts to create a room.
2. Ruse reports that all rooms are occupied.
3. User may join the room-creation queue.
4. User must keep the tab or PWA open.
5. Queue position updates in real time.

## 8.2 Join an existing room

1. User opens Ruse.
2. User enters a room code.
3. User enters a temporary display name.
4. Backend validates that the room exists, is open, and has capacity.
5. Backend issues a short-lived media access token.
6. User joins the room.
7. If a stream is active, playback begins according to browser autoplay rules.

## 8.3 Start screen sharing

1. Active stream host selects **Share screen**.
2. Browser presents the native source picker.
3. User chooses a browser tab, window, or desktop.
4. User enables audio in the browser picker when available.
5. Ruse previews the selected source.
6. User confirms publication.
7. Stream is published to the room through LiveKit.
8. All participants receive stream state updates.

## 8.4 Transfer streaming permission

1. Creator or active host opens participant actions.
2. Selects **Make stream host**.
3. Current share is stopped or gracefully handed over.
4. Backend updates stream ownership in Redis.
5. New host receives permission to publish the main share.
6. Room UI identifies the new host.

## 8.5 End a room

1. Creator selects **End room**.
2. Ruse shows a destructive confirmation.
3. Backend marks the room as closing.
4. Remaining participants receive a room-ended event.
5. LiveKit room is deleted.
6. Redis room state is deleted.
7. Room code is permanently invalidated.
8. The active room slot is released.
9. The first valid queued creator receives a temporary reservation.

---

## 9. Queue system

### 9.1 Purpose

The queue prevents more than three active rooms from being created while avoiding a first-click race when capacity becomes available.

### 9.2 Queue rules

- Queue applies only to creating a new room.
- Joining an existing room with a valid code remains allowed while queued.
- The queue is first-in, first-out.
- One browser session may hold only one queue entry.
- The waiting tab or installed PWA must remain open.
- Queue updates are sent through WebSocket.
- A heartbeat is sent approximately every 15–20 seconds.
- A disconnected user retains their position for a 60-second grace period.
- After the grace period, the queue entry is removed.
- No email or push notification is used in V1.

### 9.3 Reservation flow

When a room slot becomes available:

1. The first valid waiting entry is selected atomically.
2. The room slot is reserved for that entry.
3. The selected user receives a WebSocket event.
4. The user sees a five-minute countdown.
5. The user can create the room during the reservation.
6. If the reservation expires or the user disconnects beyond the grace period, the slot is offered to the next queued user.

### 9.4 Queue UI states

**Waiting**

- “All rooms are currently occupied.”
- Current queue position.
- Approximate wait range, clearly labelled as an estimate.
- “Keep this tab open.”
- Leave queue action.

**Reserved**

- “A room is available.”
- Five-minute countdown.
- Primary **Create private room** action.

**Disconnected**

- Reconnection indicator.
- Remaining grace-period time.

**Expired**

- Explanation that the reservation or queue session expired.
- Action to join the queue again.

---

## 10. Functional requirements

## 10.1 Room creation and codes

- Room codes must be short enough to type but sufficiently random to resist guessing.
- Codes must not be sequential.
- Codes must be case-insensitive or use an unambiguous uppercase alphabet.
- Ambiguous characters such as `0/O` and `1/I` should be excluded.
- Suggested format: 8–10 characters grouped for readability.
- Room codes must be stored as a hash where practical.
- Expired codes must never reactivate.
- A room may optionally display a separate human-readable label, but the code remains the access secret.

## 10.2 Room lifecycle

Room states:

```text
RESERVED → ACTIVE → CLOSING → ENDED
```

Additional expiry state:

```text
RESERVED → EXPIRED
```

Requirements:

- Maximum room duration: eight hours.
- Empty room cleanup delay: 60 seconds.
- Creator disconnection does not immediately end the room.
- Creator may reconnect during a defined grace period.
- If everyone leaves, the room ends after the empty-room delay.
- At maximum duration, participants receive advance warnings before termination.

## 10.3 Participant capacity

- Maximum participants per room: eight.
- The ninth join attempt is rejected with a clear room-full response.
- Reconnecting participants retain a temporary seat reservation during the reconnect grace period.
- The server is authoritative for capacity.

## 10.4 Screen-share media

- One active screen share per room.
- Target maximum resolution: 1920×1080.
- Target maximum frame rate: 30 FPS.
- Recommended target bitrate: approximately 3.5 Mbps.
- Hard client publish ceiling: approximately 5 Mbps.
- Browser-selected system/tab audio is published as a separate media track where available.
- Ruse does not record the stream.
- Ruse does not transcode the stream in V1.

## 10.5 Audio controls

Each participant must have independent controls for:

- Incoming shared-stream mute.
- Incoming shared-stream volume.
- Own microphone mute.
- Room voice output mute, if voice chat is enabled.

Muting the stream locally must not affect other participants.

## 10.6 Chat

- Real-time room text chat.
- Display name and session-generated participant identifier.
- Message timestamp.
- Basic text only in V1.
- Reasonable message length limit.
- Rate limiting and spam protection.
- No permanent message persistence after room cleanup.
- Creator may remove a participant for abuse.

## 10.7 Pin and floating player

The active stream can be pinned into a floating in-app player.

Requirements:

- Available while navigating room panels or expanding chat.
- Resizable within defined minimum and maximum dimensions.
- Draggable within viewport boundaries.
- Maintains aspect ratio.
- Includes local mute, volume, unpin, and return-to-stage controls.
- Must not obstruct critical dialogs or destructive confirmations.
- The pinned state is local to each participant.

This feature is distinct from browser Picture-in-Picture. Native Picture-in-Picture may be added as an optional enhancement where supported.

## 10.8 Fullscreen

- Stage fullscreen action expands the main media area.
- Chat and participant rail may collapse into overlays or drawers.
- Keyboard escape exits fullscreen.
- Pinning remains available before or after fullscreen.
- Controls auto-hide during playback and reappear on pointer movement.

## 10.9 PWA

- Installable manifest.
- App icons and monochrome maskable icon.
- Standalone display mode.
- Offline shell for the landing page and meaningful offline state.
- Active rooms require an internet connection.
- Service worker updates must avoid interrupting active rooms.
- Installed PWA must use the same room and capture workflow as the browser version.

---

## 11. Information architecture

### Public routes

```text
/                   Landing and create/join entry
/join               Join room form
/queue              Room-creation queue
/compatibility      Browser and capture diagnostics
/privacy            Privacy policy
/terms              Terms and acceptable-use policy
```

### Temporary room routes

```text
/room/[code]         Main room interface
/room/[code]/ended   Room-ended state
```

Room access must still be verified server-side. Knowing the route alone is not sufficient.

---

## 12. Interface specification

## 12.1 Visual direction

Ruse should use:

- Near-black application background.
- Slightly lighter neutral panels.
- Thin, low-contrast borders.
- Small-to-medium corner radii.
- White primary text.
- Muted grey secondary text.
- One restrained accent colour for active controls and focus states.
- No gradients.
- No oversized decorative illustrations inside the room.
- No excessive shadows or glowing effects.

## 12.2 Desktop room layout

The supplied layout is the primary structural reference.

```text
┌───────────────────────────────────────────────┬──────────────┐
│                                               │ Room code    │
│                                               ├──────────────┤
│                                               │ Participant  │
│                 Main stage                    │ tile         │
│                                               ├──────────────┤
│                                               │ Participant  │
│                                               │ tile         │
│                                               ├──────────────┤
│                                               │ Participant  │
│                                               │ tile         │
│                                               ├──────────────┤
│                                               │ Show all     │
│                                               ├──────────────┤
│                                               │ Room chat    │
└───────────────────────────────────────────────┴──────────────┘
```

### Main stage

- Occupies the majority of horizontal space.
- Displays empty-room instructions before sharing starts.
- Displays the active screen share using contain-fit by default.
- Provides fullscreen, pin, volume, mute, quality and connection-state controls.
- Bottom-centred control dock appears on interaction.

### Right rail

- Fixed-width sidebar on desktop.
- Room code header at the top with copy action.
- Compact participant tiles beneath the header.
- Show-all action when participant count exceeds visible slots.
- Chat occupies the lower portion.
- Rail can be collapsed to maximize the stage.

### Participant tiles

- Display participant initials or optional generated avatar.
- Display name.
- Microphone state.
- Connection state.
- Host/creator badge where applicable.
- Context menu for creator actions.

### Chat

- Header: **Room Chat**.
- Scrollable message region.
- Compact composer fixed to the bottom.
- Send icon.
- Unread indicator when chat is collapsed.

## 12.3 Responsive behaviour

### Tablet

- Stage remains primary.
- Right rail becomes a narrower collapsible panel.
- Participant tiles may become a horizontal strip.

### Mobile

Mobile is a joining and watching experience, not the primary screen-sharing environment.

- Stage at top.
- Controls below stage.
- Participants and chat available as bottom sheets or tabs.
- Room creation and joining remain supported.
- Screen sharing may be disabled or labelled experimental depending on browser capability.

---

## 13. Recommended technology stack

| Layer | Technology |
|---|---|
| Web frontend | Next.js with TypeScript |
| Styling | Tailwind CSS |
| Component primitives | Radix UI or shadcn/ui, selectively used |
| PWA | Web app manifest + service worker tooling |
| Media client | LiveKit JavaScript/React SDK |
| Media server | Self-hosted LiveKit SFU |
| Application API | Go |
| Realtime application events | Go WebSocket service |
| Ephemeral state | Redis |
| Persistent operational data | PostgreSQL |
| Reverse proxy/TLS | Nginx or Caddy |
| Deployment | Docker Compose initially |
| Metrics | Prometheus-compatible metrics + Grafana optional |

### Why LiveKit

LiveKit handles WebRTC media routing, track subscription, reconnection and SFU concerns while allowing the room lifecycle, capacity, queue, permissions and application logic to remain controlled by the Go backend.

### Why separate Go WebSockets

Application events should remain independent from media transport:

- Queue position.
- Room lifecycle events.
- Host transfer.
- Participant moderation.
- Chat, if not implemented through media data packets.
- Capacity and reservation updates.

This separation keeps media concerns and product-state concerns independently testable.

---

## 14. System architecture

```text
                         ┌─────────────────────────┐
                         │     Chromium client     │
                         │ Next.js PWA + LiveKit   │
                         └────────────┬────────────┘
                                      │
                     HTTPS / WSS      │       WebRTC
                                      │
                 ┌────────────────────▼───┐   ┌───────────────┐
                 │       Go API           │   │ LiveKit SFU   │
                 │ Room + queue + policy  │   │ Media routing │
                 └───────────┬────────────┘   └───────┬───────┘
                             │                        │
                   ┌─────────▼─────────┐      ┌──────▼──────┐
                   │       Redis       │      │ TURN/TLS    │
                   │ Ephemeral state   │      │ Connectivity│
                   └─────────┬─────────┘      └─────────────┘
                             │
                   ┌─────────▼─────────┐
                   │    PostgreSQL     │
                   │ Operational data  │
                   └───────────────────┘
```

---

## 15. Service boundaries

## 15.1 Next.js frontend

Responsibilities:

- Landing, join and queue interfaces.
- Room UI.
- Screen-capture invocation.
- LiveKit client connection.
- Local volume and mute state.
- Pin/floating player behaviour.
- PWA install and update experience.
- Capability diagnostics.

Must not be authoritative for:

- Room capacity.
- Queue ordering.
- Host permissions.
- Room termination.
- Room code validity.

## 15.2 Go API and realtime service

Responsibilities:

- Create and validate room reservations.
- Generate and validate room codes.
- Issue short-lived LiveKit access tokens.
- Enforce participant and room limits.
- Manage queue and reservations.
- Manage room ownership and stream-host permission.
- Manage participant kicks and room locks.
- Broadcast application state through WebSockets.
- Perform cleanup and recovery jobs.
- Write minimal operational records to PostgreSQL.

## 15.3 LiveKit

Responsibilities:

- Route video, shared audio and microphone tracks.
- Manage WebRTC connectivity.
- Provide track publication/subscription state.
- Support reconnect and network adaptation.
- Expose media-room administrative APIs to the Go backend.

## 15.4 Redis

Responsibilities:

- Active room records.
- Room code mapping.
- Participant presence.
- Queue order.
- Queue heartbeats.
- Slot reservations.
- Host and stream ownership.
- Temporary bans and rate-limit counters.
- Distributed locks and atomic capacity operations.

## 15.5 PostgreSQL

Responsibilities:

- Aggregate room session records.
- Session start/end times.
- Peak participant counts.
- Error categories.
- Abuse and moderation events.
- Capacity metrics.

Not stored in V1:

- Permanent chat logs.
- Screen content.
- Media recordings.
- User profiles.

---

## 16. Suggested data model

## 16.1 Redis keys

```text
ruse:rooms:active                         SET(roomId)
ruse:room:code:{codeHash}                 STRING(roomId)
ruse:room:{roomId}                        HASH
ruse:room:{roomId}:participants           HASH
ruse:room:{roomId}:banned                 SET(sessionFingerprint)
ruse:queue                                ZSET(joinedAt -> queueSessionId)
ruse:queue:{queueSessionId}               HASH
ruse:reservation:{reservationId}          HASH
ruse:capacity:lock                        distributed lock / Lua-managed state
```

### Room hash

```json
{
  "roomId": "uuid",
  "codeHash": "hash",
  "state": "ACTIVE",
  "creatorSessionId": "uuid",
  "streamHostSessionId": "uuid",
  "createdAt": "timestamp",
  "expiresAt": "timestamp",
  "locked": false,
  "participantCount": 4
}
```

### Queue session hash

```json
{
  "queueSessionId": "uuid",
  "connectionId": "uuid",
  "status": "WAITING",
  "joinedAt": "timestamp",
  "lastHeartbeatAt": "timestamp",
  "reservationId": null
}
```

## 16.2 PostgreSQL tables

### `room_sessions`

- `id UUID PRIMARY KEY`
- `started_at TIMESTAMPTZ`
- `ended_at TIMESTAMPTZ`
- `end_reason TEXT`
- `peak_participants SMALLINT`
- `duration_seconds INTEGER`
- `screen_share_started BOOLEAN`
- `turn_usage_detected BOOLEAN`
- `created_from_queue BOOLEAN`

### `operational_events`

- `id UUID PRIMARY KEY`
- `room_session_id UUID NULL`
- `event_type TEXT`
- `severity TEXT`
- `metadata JSONB`
- `created_at TIMESTAMPTZ`

Metadata must exclude screen content, chat bodies and unnecessary personal identifiers.

---

## 17. API outline

### Rooms

```text
POST   /api/v1/rooms/reserve
POST   /api/v1/rooms
POST   /api/v1/rooms/join
POST   /api/v1/rooms/{roomId}/leave
POST   /api/v1/rooms/{roomId}/end
POST   /api/v1/rooms/{roomId}/lock
POST   /api/v1/rooms/{roomId}/unlock
POST   /api/v1/rooms/{roomId}/transfer-stream-host
POST   /api/v1/rooms/{roomId}/participants/{participantId}/remove
GET    /api/v1/rooms/{roomId}/state
```

### Queue

```text
POST   /api/v1/queue/join
POST   /api/v1/queue/heartbeat
POST   /api/v1/queue/leave
GET    /api/v1/queue/status
POST   /api/v1/queue/reservations/{reservationId}/claim
```

### Media access

```text
POST   /api/v1/media/token
```

### Diagnostics

```text
GET    /api/v1/health
GET    /api/v1/capacity
```

Public capacity responses should be coarse and must not expose room codes or participant details.

---

## 18. WebSocket events

### Server to client

```text
room.state.changed
room.ended
room.locked
room.unlocked
participant.joined
participant.left
participant.removed
participant.updated
stream.host.changed
stream.started
stream.stopped
chat.message.created
queue.position.changed
queue.reserved
queue.reservation.expiring
queue.reservation.expired
queue.removed
system.warning
```

### Client to server

```text
presence.heartbeat
chat.message.send
queue.heartbeat
room.state.request
```

Every event should use a versioned envelope:

```json
{
  "version": 1,
  "type": "queue.position.changed",
  "timestamp": "2026-07-16T10:00:00Z",
  "requestId": "uuid",
  "payload": {}
}
```

---

## 19. Atomic capacity management

The three-room global limit must be enforced atomically in Redis.

Never implement capacity as:

1. Read active room count.
2. Check whether it is below three.
3. Create a room separately.

That flow permits concurrent over-allocation.

Use a Redis Lua script or transaction that performs these operations as one unit:

- Remove stale reservations.
- Count active rooms and valid reservations.
- Reject when total capacity equals three.
- Create a temporary reservation when capacity exists.
- Set reservation expiry.

When a room ends, another atomic operation should:

- Remove the active room.
- Invalidate its code.
- Select the first valid queue entry.
- Create a five-minute slot reservation.
- Update queue state.

---

## 20. Security and privacy

## 20.1 Room access

- Room codes generated using a cryptographically secure random source.
- Rate-limit code attempts by IP and temporary browser session.
- Introduce progressive cooldown after repeated invalid attempts.
- Do not reveal whether a similarly formatted expired code previously existed.
- Optional creator-controlled room lock.

## 20.2 Session identity

Because there are no accounts:

- Each browser receives a signed, short-lived session token.
- Tokens are bound to room and role claims.
- LiveKit tokens are short-lived and issued server-side.
- Creator recovery uses a separate creator secret stored in session storage or secure browser storage.
- Sensitive tokens must never appear in query parameters or logs.

## 20.3 Transport security

- HTTPS and secure WebSockets only.
- Valid TLS certificates.
- TURN over TLS as fallback.
- Strict origin validation.
- Secure cookie attributes where cookies are used.
- Content Security Policy.

## 20.4 Abuse prevention

- Message rate limits.
- Room creation rate limits.
- Queue join rate limits.
- Maximum display-name length.
- Text sanitization.
- Creator moderation controls.
- Temporary participant bans scoped to the active room.

## 20.5 Privacy

- No media recording.
- No permanent chat storage.
- No user account database.
- Minimal IP retention, limited to security logs and shortened retention.
- Clear consent and capture indicators.
- Clear policy that users may only share content they are authorized to display.

---

## 21. Failure handling

## 21.1 Host stops sharing

- Stage displays “The host stopped sharing.”
- Room remains active.
- Active host can restart sharing.
- Creator can transfer stream permission.

## 21.2 Host disconnects

- Keep host role reserved briefly during reconnect.
- Display reconnecting state.
- If grace period expires, creator can nominate another stream host.
- If creator was also disconnected, room remains active until empty or maximum duration is reached.

## 21.3 Viewer disconnects

- Reserve participant seat during short reconnect grace period.
- Restore chat and participant state from current ephemeral room state.
- Local UI preferences may be restored from browser storage.

## 21.4 Redis restart

- Redis persistence configuration should preserve short-lived room metadata where possible.
- Go service should reconcile Redis rooms with LiveKit rooms on startup.
- Orphaned LiveKit rooms should be closed or reconstructed according to reconciliation policy.

## 21.5 Go service restart

- Clients reconnect WebSockets with exponential backoff.
- Active media may continue temporarily through LiveKit.
- Application state is restored from Redis.

## 21.6 LiveKit restart

- Clients display media reconnect state.
- Go service prevents new room creation until media health is restored.
- Existing rooms receive a clear failure message if reconnection cannot succeed.

## 21.7 Capacity queue failure

- Queue state must be server-authoritative.
- Client position is informational only.
- Expired or duplicate queue sessions are removed automatically.
- Capacity reconciliation runs periodically.

---

## 22. Performance and deployment limits

### Initial hard limits

```yaml
active_rooms_global: 3
participants_per_room: 8
screen_shares_per_room: 1
room_max_duration: 8h
empty_room_grace: 60s
queue_disconnect_grace: 60s
queue_reservation_duration: 5m
screen_share_max_resolution: 1920x1080
screen_share_max_fps: 30
screen_share_target_bitrate: 3.5Mbps
screen_share_max_bitrate: 5Mbps
```

### Estimated worst configured media fan-out

Three rooms, each with one host and seven viewers:

- 3 publishing screen-share hosts.
- 21 screen-share subscribers.
- Up to 24 participants total.

The server should be tested under TURN-heavy and packet-loss conditions before public release.

---

## 23. VPS deployment topology

```text
VPS: 4 vCPU / 8 GB RAM

├── Reverse proxy
│   ├── app.ruse-domain.tld
│   ├── api.ruse-domain.tld
│   └── livekit.ruse-domain.tld
├── Next.js frontend
├── Go API + WebSocket service
├── LiveKit server
├── Redis
├── PostgreSQL
└── Monitoring agents
```

### Deployment requirements

- LiveKit should be independently deployable even if hosted on the same VPS initially.
- Media ports and TURN ports must be correctly exposed through the VPS firewall.
- LiveKit should use host networking where appropriate for the selected deployment model.
- PostgreSQL should not be exposed publicly.
- Redis should not be exposed publicly.
- Administrative metrics endpoints should be private or authenticated.

---

## 24. Observability

Track at minimum:

### Product metrics

- Active rooms.
- Room creation attempts.
- Queue joins.
- Queue abandonment rate.
- Queue wait time.
- Reservation claim rate.
- Average room duration.
- Peak participants per room.

### Media metrics

- Active publishers and subscribers.
- Outbound bitrate.
- Packet loss.
- Jitter.
- Retransmissions.
- TURN-relayed sessions.
- Reconnection rate.

### Infrastructure metrics

- CPU usage.
- Memory usage.
- Network ingress and egress.
- Redis memory and latency.
- PostgreSQL connection count.
- WebSocket connection count.
- Container restart count.

### Alert conditions

- CPU sustained above safe threshold.
- Network saturation.
- LiveKit unhealthy.
- Redis unavailable.
- PostgreSQL unavailable.
- Active-room count inconsistency.
- Queue reservation stuck beyond expiry.

---

## 25. Accessibility

- Full keyboard navigation.
- Visible focus states.
- Semantic buttons and labels.
- Accessible names for icon-only controls.
- Sufficient colour contrast.
- Chat announcements should avoid excessive screen-reader interruption.
- Captions are not generated by Ruse in V1, but the UI must not interfere with captions present in shared content.
- Reduced-motion preference respected.

---

## 26. Testing strategy

## 26.1 Frontend tests

- Room creation and join forms.
- Capture-state transitions.
- Local audio controls.
- Floating-player drag and bounds.
- Fullscreen layout.
- Queue countdown and reconnect states.
- PWA install and service-worker update flows.

## 26.2 Go unit tests

- Room code generation and validation.
- Room state machine.
- Capacity enforcement.
- Queue ordering.
- Reservation expiry.
- Creator and stream-host permissions.
- Participant limits.
- Token claims.

## 26.3 Integration tests

- Redis atomic room reservation.
- Room termination releasing capacity.
- Queue promotion after room termination.
- LiveKit room creation and deletion.
- Reconciliation after service restart.
- WebSocket reconnect and missed-state recovery.

## 26.4 End-to-end tests

- Creator creates a room and shares a tab with audio.
- Seven participants join.
- Participant mutes stream locally.
- Participant pins the stream.
- Creator transfers stream host.
- Creator ends the room.
- Queued creator receives and claims the newly available slot.
- Old room code no longer works.

## 26.5 Load tests

Test at least:

- Three simultaneous rooms.
- Eight participants per room.
- Three concurrent 1080p/30 screen shares.
- Chat traffic during media load.
- WebSocket reconnect storms.
- TURN-heavy connectivity.
- Host network degradation.

---

## 27. Acceptance criteria

Ruse V1 is ready for release when:

1. A user can create a room without an account when capacity is available.
2. A user can join only with a valid active room code.
3. An expired room code cannot be reused.
4. A host can share a selected surface and include audio when exposed by the browser.
5. Up to seven viewers can watch the host stream in the same room.
6. Every participant can mute and control shared-stream audio locally.
7. Every participant can use chat in real time.
8. Every participant can pin the active stream into a floating in-app player.
9. The creator can transfer stream-host permission.
10. The creator can remove a participant and end the room.
11. Ending a room deletes temporary state and releases capacity.
12. No more than three active rooms plus valid capacity reservations can exist.
13. A fourth creator can join the queue.
14. Queue position updates while the tab remains open.
15. The first queued user receives a five-minute reservation when capacity opens.
16. The PWA can be installed from a supported Chromium browser.
17. The interface structurally matches the supplied dark stage-and-sidebar layout.
18. The platform does not record or permanently store shared media or chat.

---

## 28. Delivery phases

## Phase 1 — Foundation

- Monorepo or clearly separated frontend/backend repositories.
- Next.js shell and design tokens.
- Go API scaffold.
- Redis and PostgreSQL setup.
- LiveKit local development environment.
- Health checks and Docker Compose.

## Phase 2 — Room lifecycle

- Secure temporary session identity.
- Room creation and joining.
- Private room codes.
- Participant capacity.
- Room expiry and cleanup.
- LiveKit token generation.

## Phase 3 — Media room

- Screen capture.
- Shared-surface audio.
- Stream publication and playback.
- Local stream mute and volume.
- Participant rail.
- Reconnection states.

## Phase 4 — Collaboration

- Room chat.
- Host transfer.
- Participant removal.
- Room locking.
- End-room flow.

## Phase 5 — Advanced interface

- Floating pinned player.
- Fullscreen stage.
- Responsive side panels.
- Participant overflow view.
- Capture diagnostics.

## Phase 6 — Global capacity queue

- Atomic three-room limit.
- In-app FIFO queue.
- Heartbeat and disconnect grace period.
- Five-minute slot reservation.
- Queue-to-room transition.

## Phase 7 — PWA and production hardening

- Installability.
- Service worker strategy.
- Security headers.
- Rate limits.
- Metrics and alerts.
- Load tests.
- VPS deployment.

---

## 29. Future possibilities

These are explicitly outside V1 and should not affect the first architecture unless stated:

- Optional native desktop companion for broader capture capabilities.
- Browser Picture-in-Picture integration.
- End-to-end encrypted chat payloads.
- Scheduled rooms.
- Invite links in addition to codes.
- Shared playback of user-owned local files without upload.
- Optional voice activity view.
- Multiple regional media servers.
- More than three active rooms after infrastructure separation.
- Accounts for trusted recurring groups.

---

## 30. Final product definition

Ruse V1 is a Chromium-first, installable private watch-room PWA for a maximum of three simultaneous ephemeral rooms. Each room supports one screen-share host and up to seven viewers, shared-surface audio where browser capabilities allow it, local audio controls, room chat, fullscreen viewing, an in-app floating pinned player, temporary display-name identities, private short-lived codes, host transfer, and automatic cleanup.

The Next.js client handles interface and capture. A Go backend owns room policy, queueing, capacity, permissions and lifecycle. LiveKit routes media. Redis stores ephemeral state and enforces atomic capacity. PostgreSQL stores only minimal operational records. No login, public discovery, permanent chat history, content hosting or recording is included.