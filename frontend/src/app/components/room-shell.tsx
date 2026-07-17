"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ConnectionState, Participant, Room, RoomEvent, Track, TrackPublication } from "livekit-client";
import { HugeiconsIcon } from "@hugeicons/react";
import { ComputerScreenShareIcon, FullScreenIcon, Mic01Icon, MicOff01Icon, PinIcon, PinOffIcon, SentIcon, Video01Icon, VideoOffIcon } from "@hugeicons/core-free-icons";
import { decideJoin, endRoom, getMediaAccess, getRoomState, listPendingRequests, PendingRequest, RoomState } from "../lib/api";

type Props = { roomId: string; code?: string; isOwner?: boolean; onHome: () => void };
type Message = { sender: string; body: string };

const screenShare1080p60 = { maxBitrate: 10_000_000, maxFramerate: 60 };
const screenCapture1080p60 = { width: 1920, height: 1080, frameRate: 60 };

function VideoTile({ participant, self, pinned, onPin }: { participant: Participant; self?: boolean; pinned?: boolean; onPin?: () => void }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const publication = participant.getTrackPublication(Track.Source.Camera) as TrackPublication | undefined;
  const track = publication?.videoTrack;

  useEffect(() => {
    const video = videoRef.current;
    if (!track || !video) return;
    track.attach(video);
    return () => { track.detach(video); };
  }, [track]);

  return <article className={`relative aspect-video overflow-hidden rounded-md bg-[#111827] ${participant.isSpeaking ? "ring-2 ring-emerald-400" : "ring-1 ring-white/10"}`}>
    {track ? <video ref={videoRef} autoPlay muted={self} playsInline className="size-full object-cover" /> : <div className="grid size-full place-items-center text-xs text-white/45">Camera off</div>}
    <span className="absolute bottom-1 right-1 rounded bg-black/70 px-1.5 py-0.5 text-[10px]">{self ? "You" : participant.name || "Guest"}</span>
    {onPin && <button onClick={onPin} aria-label={pinned ? "Unpin video" : "Pin video"} className="absolute right-1 top-1 grid size-6 place-items-center rounded bg-black/70"><HugeiconsIcon icon={pinned ? PinOffIcon : PinIcon} size={14} /></button>}
  </article>;
}

export function RoomShell({ roomId, code, isOwner = false, onHome }: Props) {
  const [roomState, setRoomState] = useState<RoomState | null>(null);
  const [pending, setPending] = useState<PendingRequest[]>([]);
  const [room, setRoom] = useState<Room | null>(null);
  const [members, setMembers] = useState<Participant[]>([]);
  const [canScreenShare, setCanScreenShare] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [cameraOn, setCameraOn] = useState(false);
  const [micOn, setMicOn] = useState(false);
  const [pinnedIdentity, setPinnedIdentity] = useState<string | null>(null);
  const [isFullscreen, setFullscreen] = useState(false);
  const [audioBlocked, setAudioBlocked] = useState(false);
  const [chatHidden, setChatHidden] = useState(false);
  const [pinnedPosition, setPinnedPosition] = useState<{ left: number; top: number } | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const stageRef = useRef<HTMLDivElement>(null);
  const stageContainerRef = useRef<HTMLElement>(null);
  const pinnedRef = useRef<HTMLDivElement>(null);
  const pinnedDragRef = useRef<{ pointerId: number; startX: number; startY: number; left: number; top: number } | null>(null);
  const audioRef = useRef<HTMLDivElement>(null);
  const screenAudioElementRef = useRef<HTMLAudioElement | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [state, requests] = await Promise.all([getRoomState(roomId), isOwner ? listPendingRequests(roomId) : Promise.resolve({ requests: [] })]);
      setRoomState(state);
      setPending(requests.requests);
    } catch {
      setError("Room state could not be refreshed.");
    }
  }, [isOwner, roomId]);

  useEffect(() => {
    const initial = window.setTimeout(refresh, 0);
    const interval = window.setInterval(refresh, 4000);
    return () => { window.clearTimeout(initial); window.clearInterval(interval); };
  }, [refresh]);

  useEffect(() => {
    const liveRoom = new Room({ publishDefaults: { screenShareEncoding: screenShare1080p60 } });
    let disposed = false;
    const syncFullscreen = () => setFullscreen(document.fullscreenElement === stageContainerRef.current);
    const sync = () => setMembers([liveRoom.localParticipant, ...Array.from(liveRoom.remoteParticipants.values())]);
    const clearScreenAudio = () => {
      const element = screenAudioElementRef.current;
      if (!element) return;
      element.pause();
      element.srcObject = null;
      element.remove();
      screenAudioElementRef.current = null;
    };

    document.addEventListener("fullscreenchange", syncFullscreen);
    liveRoom.on(RoomEvent.ParticipantConnected, sync).on(RoomEvent.ParticipantDisconnected, sync).on(RoomEvent.AudioPlaybackStatusChanged, (canPlay) => setAudioBlocked(!canPlay)).on(RoomEvent.TrackSubscribed, (track) => {
      if (track.kind === Track.Kind.Audio && audioRef.current) {
        if (track.source === Track.Source.ScreenShareAudio) clearScreenAudio();
        const element = track.attach() as HTMLAudioElement;
        element.autoplay = true;
        audioRef.current.append(element);
        if (track.source === Track.Source.ScreenShareAudio) screenAudioElementRef.current = element;
      }
      if (track.source === Track.Source.ScreenShare && stageRef.current) {
        const element = track.attach() as HTMLVideoElement;
        element.autoplay = true;
        element.playsInline = true;
        element.className = "size-full max-h-full max-w-full object-contain";
        stageRef.current.replaceChildren(element);
      }
      sync();
    }).on(RoomEvent.TrackUnsubscribed, (track) => {
      if (track.kind === Track.Kind.Audio) {
        track.detach().forEach((element) => element.remove());
        if (track.source === Track.Source.ScreenShareAudio) screenAudioElementRef.current = null;
      }
      if (track.source === Track.Source.ScreenShare && stageRef.current) stageRef.current.replaceChildren("Waiting for a shared screen");
      sync();
    }).on(RoomEvent.LocalTrackPublished, sync).on(RoomEvent.LocalTrackUnpublished, (publication) => {
      if (publication.source === Track.Source.Camera) setCameraOn(false);
      if (publication.source === Track.Source.ScreenShare) setSharing(false);
      sync();
    }).on(RoomEvent.TrackMuted, sync).on(RoomEvent.TrackUnmuted, sync).on(RoomEvent.ActiveSpeakersChanged, sync).on(RoomEvent.Disconnected, () => {
      setRoom(null);
      setError("Media connection closed. Reload the room to reconnect.");
    }).on(RoomEvent.DataReceived, (data, participant) => {
      const body = new TextDecoder().decode(data);
      if (body) setMessages((items) => [...items, { sender: participant?.name || "Guest", body }]);
    });

    getMediaAccess(roomId, isOwner ? "owner" : "guest").then(async (access) => {
      if (disposed) return;
      await liveRoom.connect(access.url, access.token);
      if (disposed) { liveRoom.disconnect(); return; }
      setCanScreenShare(access.canPublish);
      setRoom(liveRoom);
      sync();
    }).catch((cause) => {
      if (!disposed) setError(cause instanceof Error ? `Media connection failed: ${cause.message}` : "Media connection failed.");
    });

    return () => { disposed = true; document.removeEventListener("fullscreenchange", syncFullscreen); clearScreenAudio(); liveRoom.disconnect(); };
  }, [isOwner, roomId]);

  const connected = room?.state === ConnectionState.Connected;

  async function toggleCamera() {
    if (!connected || !room) return;
    try {
      await room.localParticipant.setCameraEnabled(!cameraOn);
      setCameraOn((value) => !value);
      setMembers([room.localParticipant, ...Array.from(room.remoteParticipants.values())]);
    } catch {
      setError("Camera permission was not granted.");
    }
  }

  async function toggleMic() {
    if (!connected || !room) return;
    try {
      await room.localParticipant.setMicrophoneEnabled(!micOn);
      setMicOn((value) => !value);
    } catch {
      setError("Microphone permission was not granted.");
    }
  }

  async function shareScreen() {
    if (!connected || !room || !canScreenShare || sharing) return;
    try {
      const tracks = await room.localParticipant.createScreenTracks({
        audio: true,
        contentHint: "detail",
        resolution: screenCapture1080p60,
        selfBrowserSurface: "exclude",
        surfaceSwitching: "include",
        systemAudio: "include",
      });
      await Promise.all(tracks.map((track) => room.localParticipant.publishTrack(track)));
      const screen = tracks.find((track) => track.kind === Track.Kind.Video);
      if (stageRef.current && screen) {
        const element = screen.attach() as HTMLVideoElement;
        element.autoplay = true;
        element.muted = true;
        element.className = "size-full max-h-full max-w-full object-contain";
        stageRef.current.replaceChildren(element);
      }
      setSharing(true);
      if (!tracks.some((track) => track.kind === Track.Kind.Audio)) setError("Screen is sharing without audio. Enable Share audio in the browser picker.");
    } catch {
      setError("Screen sharing was not granted.");
    }
  }

  async function fullscreen() {
    try {
      if (document.fullscreenElement === stageContainerRef.current) await document.exitFullscreen();
      else await stageContainerRef.current?.requestFullscreen();
    } catch {
      setError("Fullscreen is unavailable in this browser.");
    }
  }

  async function enableSharedAudio() {
    if (!room) return;
    try {
      await room.startAudio();
      setAudioBlocked(false);
    } catch {
      setError("Shared audio needs a click to start.");
    }
  }

  async function sendMessage(event: React.FormEvent) {
    event.preventDefault();
    const body = draft.trim();
    if (!body || !connected || !room) return;
    try {
      await room.localParticipant.publishData(new TextEncoder().encode(body), { reliable: true });
      setMessages((items) => [...items, { sender: "You", body }]);
      setDraft("");
    } catch {
      setError("Chat is unavailable until media reconnects.");
    }
  }

  async function decide(id: string, decision: "approve" | "reject") {
    try { await decideJoin(roomId, id, decision); refresh(); } catch { setError("Could not update that request."); }
  }

  async function closeRoom() {
    try { await endRoom(roomId); refresh(); } catch { setError("The room could not be ended."); }
  }

  function startPinnedDrag(event: React.PointerEvent<HTMLButtonElement>) {
    const stage = stageContainerRef.current;
    const tile = pinnedRef.current;
    if (!stage || !tile) return;
    const stageBounds = stage.getBoundingClientRect();
    const tileBounds = tile.getBoundingClientRect();
    pinnedDragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      left: pinnedPosition?.left ?? tileBounds.left - stageBounds.left,
      top: pinnedPosition?.top ?? tileBounds.top - stageBounds.top,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function movePinnedDrag(event: React.PointerEvent<HTMLButtonElement>) {
    const drag = pinnedDragRef.current;
    const stage = stageContainerRef.current;
    const tile = pinnedRef.current;
    if (!drag || drag.pointerId !== event.pointerId || !stage || !tile) return;
    setPinnedPosition({
      left: Math.min(Math.max(0, drag.left + event.clientX - drag.startX), Math.max(0, stage.clientWidth - tile.offsetWidth)),
      top: Math.min(Math.max(0, drag.top + event.clientY - drag.startY), Math.max(0, stage.clientHeight - tile.offsetHeight)),
    });
  }

  function endPinnedDrag(event: React.PointerEvent<HTMLButtonElement>) {
    if (pinnedDragRef.current?.pointerId !== event.pointerId) return;
    pinnedDragRef.current = null;
    event.currentTarget.releasePointerCapture(event.pointerId);
  }

  if (roomState?.state === "ended") return <main className="grid min-h-dvh place-items-center bg-black text-white"><button onClick={onHome}>Room closed — return home</button></main>;

  const pinned = members.find((member) => member.identity === pinnedIdentity);
  const shellLayout = chatHidden ? "grid-rows-1 lg:grid-cols-1" : "grid-rows-[minmax(0,1fr)_minmax(0,1fr)] lg:grid-cols-[minmax(0,1fr)_17.5rem] lg:grid-rows-1";
  const railLayout = chatHidden ? "absolute inset-y-0 right-0 z-10 w-56" : "";
  const participantRailLayout = chatHidden ? "min-h-0 flex-1" : "h-40 shrink-0 sm:h-56 lg:h-[19rem]";
  return <main className="h-dvh max-h-dvh overflow-hidden bg-black p-2 text-white">
    <div className={`relative grid h-[calc(100dvh-1rem)] min-h-0 max-h-[calc(100dvh-1rem)] overflow-hidden rounded-md border border-white/15 bg-[#171717] ${shellLayout}`}>
      <section ref={stageContainerRef} className="relative min-h-0 overflow-hidden bg-black">
        <div ref={stageRef} className="grid size-full place-items-center text-sm text-white/45">Waiting for a shared screen</div>
        <div ref={audioRef} className="hidden" />
        {pinned && <div ref={pinnedRef} style={pinnedPosition ?? undefined} className={`absolute z-20 h-36 w-64 min-h-28 min-w-40 max-h-[80%] max-w-[80%] resize overflow-hidden rounded-md border border-white/25 shadow-xl ${pinnedPosition ? "" : "bottom-5 right-5"}`}><VideoTile participant={pinned} self={pinned === room?.localParticipant} pinned onPin={() => { setPinnedIdentity(null); setPinnedPosition(null); }} /><button onPointerDown={startPinnedDrag} onPointerMove={movePinnedDrag} onPointerUp={endPinnedDrag} onPointerCancel={endPinnedDrag} aria-label="Drag pinned video" className="absolute left-1/2 top-1 z-10 h-5 w-10 -translate-x-1/2 cursor-grab touch-none rounded bg-black/70 active:cursor-grabbing" /></div>}
        <button onClick={fullscreen} aria-label={isFullscreen ? "Exit fullscreen" : "Enter fullscreen"} className="absolute right-5 top-5 grid size-10 place-items-center rounded-md bg-black/70"><HugeiconsIcon icon={FullScreenIcon} size={21} /></button>
        <div className="absolute inset-x-0 bottom-5 flex justify-center gap-2">
          <button onClick={shareScreen} disabled={!canScreenShare || sharing} aria-label="Share screen" className="grid size-10 place-items-center rounded-md bg-black/70 disabled:opacity-35"><HugeiconsIcon icon={ComputerScreenShareIcon} size={21} /></button>
          <button onClick={toggleMic} aria-label="Toggle microphone" className="grid size-10 place-items-center rounded-md bg-black/70"><HugeiconsIcon icon={micOn ? Mic01Icon : MicOff01Icon} size={21} /></button>
          <button onClick={toggleCamera} aria-label="Toggle camera" className="grid size-10 place-items-center rounded-md bg-black/70"><HugeiconsIcon icon={cameraOn ? Video01Icon : VideoOffIcon} size={21} /></button>
          {audioBlocked && <button onClick={enableSharedAudio} className="rounded-md bg-black/70 px-3 text-sm">Enable shared audio</button>}
        </div>
      </section>
      <aside className={`flex min-h-0 max-h-full flex-col gap-2 overflow-hidden border-l border-white/15 bg-[#181818] p-2 ${railLayout}`}>
        <section className="rounded-md border border-white/10 px-3 py-2 text-sm">{isOwner ? <>Room Code: {code}</> : "You’re in this room"}</section>
        {isOwner && pending.length > 0 && <section className="space-y-1">{pending.map((request) => <div className="rounded bg-white/5 p-2" key={request.id}>{request.name}<div className="mt-1 grid grid-cols-2 gap-1"><button onClick={() => decide(request.id, "approve")} className="bg-blue-500 py-1 rounded cursor-pointer active:scale-95 transition-all duration-300">Allow</button><button onClick={() => decide(request.id, "reject")} className="bg-red-500 py-1 rounded cursor-pointer active:scale-95 transition-all duration-300">Decline</button></div></div>)}</section>}
        <section aria-label="Participant videos" className={`grid grid-cols-1 content-start gap-1.5 overflow-y-auto rounded-md border border-white/10 p-1.5 ${participantRailLayout}`}>{members.map((member) => <VideoTile key={member.identity} participant={member} self={member === room?.localParticipant} pinned={member.identity === pinnedIdentity} onPin={() => { setPinnedIdentity((identity) => identity === member.identity ? null : member.identity); setPinnedPosition(null); }} />)}</section>
        <button onClick={() => setChatHidden((hidden) => !hidden)} aria-expanded={!chatHidden} className="h-8 rounded border border-white/20 text-sm">{chatHidden ? "Show chat" : "Hide chat"}</button>
        {!chatHidden && <section className="flex min-h-0 flex-1 flex-col rounded-md border border-white/10 p-2"><p className="text-center text-sm font-medium">Room Chat</p><div className="flex-1 space-y-1 overflow-y-auto py-2">{messages.map((message, index) => <p className="rounded bg-white/5 px-2 py-1 text-xs" key={index}>{message.sender}: {message.body}</p>)}</div><form onSubmit={sendMessage} className="flex h-10 gap-2 rounded bg-white/10 px-2"><input className="min-w-0 flex-1 bg-transparent text-sm outline-none" value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="Type here" /><button aria-label="Send message"><HugeiconsIcon icon={SentIcon} size={20} /></button></form></section>}
        <p className="text-xs text-rose-300">{error}</p>
        {isOwner ? <button onClick={closeRoom} className="h-10 rounded bg-red-500">End room</button> : <button onClick={onHome} className="h-10 rounded border border-white/20">Leave room</button>}
      </aside>
    </div>
  </main>;
}
