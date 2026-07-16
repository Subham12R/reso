# Room Media Controls Design

## Goal

Make screen sharing default to 1080p at 60 fps, reliably resume shared audio after a browser autoplay block, toggle fullscreen both ways, allow the pinned camera tile to be resized from its corners, and let users collapse chat without losing participant access.

## Scope

Only `frontend/src/app/components/room-shell.tsx` changes.

- Request and publish screen video at 1920 x 1080, 60 fps with a matching LiveKit encoding cap.
- Attach screen-share audio as before, but surface an enable-audio action when the browser blocks autoplay; its click calls LiveKit's `startAudio`.
- Track the browser's `fullscreenchange` event so one button enters and exits fullscreen.
- Use the platform `resize: both` behavior on the pinned video tile, with minimum and maximum bounds.
- Add a chat-collapse control. Collapsing removes chat only; the right rail and its participant tiles remain, and the stage receives the released width.

## Constraints

- No new dependencies or backend/API changes.
- Browser capture remains subject to the selected source and browser/device support; 60 fps is requested rather than guaranteed.
- Keep existing room controls and owner moderation behavior unchanged.

## Verification

- A focused component test covers fullscreen toggle state, blocked-audio recovery visibility, and chat collapse behavior.
- `npm run lint` and `npm run build` pass in `frontend`.
