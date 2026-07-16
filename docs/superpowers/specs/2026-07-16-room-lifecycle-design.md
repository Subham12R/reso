# Reso room lifecycle design

## Scope

Build the API-first room-access workflow:

1. An owner creates a temporary room.
2. A guest submits a room code and display name.
3. The owner approves or rejects the pending request.
4. Pending requests expire after 2 minutes.

## Security

- Generate room codes and session tokens with `crypto/rand`.
- Store only hashes of room codes and session tokens.
- Return the room code only when the owner creates the room.
- Require the owner session for approval or rejection.
- Use generic responses for invalid, expired, or unavailable room codes.
- Do not log room codes or session tokens.
- Rate-limit room creation and join-request attempts.
- Send session tokens only in `HttpOnly`, `Secure`, `SameSite=Lax` cookies; never return them in JSON.


## First endpoints

- `POST /api/v1/rooms`
- `POST /api/v1/rooms/join-requests`
- `POST /api/v1/rooms/{roomId}/join-requests/{requestId}/approve`

## Out of scope

Redis wiring, frontend room UI, LiveKit, chat, and capacity queues.
