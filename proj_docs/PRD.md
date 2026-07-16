# Reso Product Requirements

## Product

Reso is a private, temporary browser room for sharing a screen with a small group. Rooms have no accounts, no recording, and require the owner to approve each join request.

## V1 goals

- Create a room with a cryptographically random code.
- Keep room codes and session secrets hashed at rest.
- Allow guests to request entry; only the owner can approve or reject them.
- Limit active rooms globally and enforce room capacity server-side.
- End rooms automatically after their maximum lifetime or explicitly by the owner.
- Provide application-state events through WebSockets and media through LiveKit.

## Users and roles

- **Creator:** creates, ends, locks, and moderates a room.
- **Stream host:** may publish the single active screen share.
- **Participant:** views shared media, sends chat, and may request stream-host permission.

## Functional requirements

### Room access

- A room code is random, non-sequential, and never stored in plaintext.
- A guest submits a display name and room code, then waits for approval.
- Rejected, expired, or ended rooms cannot be entered.

### Lifecycle and capacity

- A room is `active` until ended by its owner or its eight-hour lifetime expires.
- Global room capacity is three active rooms in the current deployment.
- Per-room participant capacity is eight; a ninth participant is rejected.
- Empty rooms should end after a 60-second grace period.

### Realtime and media

- Clients receive room state, join-request, participant, and host events over WebSockets.
- LiveKit transports screen share, shared audio, and optional microphone audio.
- One screen-share publisher is permitted per room.

## Non-goals for V1

- Accounts or social profiles.
- Recording, transcription, or permanent chat history.
- Media transcoding by Reso.

## Acceptance checks

- An owner can create and end a room.
- A guest cannot join until approved by that room's owner.
- Secrets are never returned after initial issuance or stored un-hashed.
- Active room capacity is atomically enforced in Redis.
