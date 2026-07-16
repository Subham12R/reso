"use client";

import { Room, RoomEvent, Track } from "livekit-client";
import { useEffect, useRef, useState } from "react";
import { ApiError, getMediaAccess } from "../lib/api";

type Props = { roomId: string };

export function LiveRoom({ roomId }: Props) {
  const stageRef = useRef<HTMLDivElement>(null);
  const roomRef = useRef<Room | null>(null);
  const [status, setStatus] = useState("Connecting media service…");
  const [sharing, setSharing] = useState(false);
  const [microphone, setMicrophone] = useState(false);
  const [canPublish, setCanPublish] = useState(false);

  useEffect(() => {
    const room = new Room();
    roomRef.current = room;
    const attach = (track: Track) => {
      if (track.kind !== Track.Kind.Video || !stageRef.current) return;
      const element = track.attach() as HTMLVideoElement;
      element.autoplay = true;
      element.playsInline = true;
      element.className = "live-video";
      stageRef.current.replaceChildren(element);
    };
    room.on(RoomEvent.TrackSubscribed, attach);
    room.on(RoomEvent.TrackUnsubscribed, (track) => track.detach().forEach((element) => element.remove()));

    getMediaAccess(roomId)
      .then(async ({ url, token, canPublish: publish }) => {
        setCanPublish(publish);
        await room.connect(url, token);
        setStatus(publish ? "Ready to share your screen." : "Watching the shared screen.");
      })
      .catch((cause) => setStatus(cause instanceof ApiError && cause.status === 503 ? "Media service is not configured." : "Media service is unavailable."));

    return () => { room.disconnect(); roomRef.current = null; };
  }, [roomId]);

  async function shareScreen() {
    const room = roomRef.current;
    if (!room || !canPublish) return;
    try {
      const [track] = await room.localParticipant.createScreenTracks({ audio: false });
      track.mediaStreamTrack.addEventListener("ended", () => { setSharing(false); });
      await room.localParticipant.publishTrack(track);
      if (stageRef.current && track.kind === Track.Kind.Video) {
        const element = track.attach() as HTMLVideoElement;
        element.autoplay = true; element.muted = true; element.playsInline = true; element.className = "live-video";
        stageRef.current.replaceChildren(element);
      }
      setSharing(true);
    } catch { setStatus("Screen sharing could not start. Check browser permission."); }
  }

  async function toggleMicrophone() {
    const room = roomRef.current;
    if (!room) return;
    try { await room.localParticipant.setMicrophoneEnabled(!microphone); setMicrophone((value) => !value); }
    catch { setStatus("Microphone access was not granted."); }
  }

  async function fullscreen() { await stageRef.current?.requestFullscreen?.(); }

  return <>
    <div ref={stageRef} className="stage-empty"><span className="stage-mark">R</span><h1>The stage is quiet.</h1><p>{status}</p></div>
    <div className="stage-controls" aria-label="Media controls">
      <button onClick={shareScreen} disabled={!canPublish || sharing}>{sharing ? "Sharing screen" : "Share screen"}</button>
      <button onClick={toggleMicrophone}>{microphone ? "Mute microphone" : "Microphone"}</button>
      <button onClick={fullscreen}>Fullscreen</button>
    </div>
  </>;
}
