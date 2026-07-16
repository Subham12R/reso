"use client";

import { useEffect, useState } from "react";
import { ApiError, getGuestJoinStatus } from "../lib/api";
import { EntryPanel } from "./entry-panel";
import { RoomShell } from "./room-shell";

type View =
  | { kind: "entry"; mode: "create" | "join" }
  | { kind: "owner"; roomId: string; code: string }
  | { kind: "guest"; roomId: string }
  | { kind: "waiting"; requestId: string };

export function ResoApp() {
  const [view, setView] = useState<View>({ kind: "entry", mode: "create" });

  if (view.kind === "owner") return <RoomShell roomId={view.roomId} code={view.code} isOwner onHome={() => setView({ kind: "entry", mode: "create" })} />;
  if (view.kind === "guest") return <RoomShell roomId={view.roomId} onHome={() => setView({ kind: "entry", mode: "join" })} />;
  if (view.kind === "waiting") return <GuestWaiting requestId={view.requestId} onApproved={(roomId) => setView({ kind: "guest", roomId })} onBack={() => setView({ kind: "entry", mode: "join" })} />;

  return <EntryPanel initialMode={view.mode} onCreated={(roomId, code) => setView({ kind: "owner", roomId, code })} onRequested={(requestId) => setView({ kind: "waiting", requestId })} />;
}

function GuestWaiting({ requestId, onApproved, onBack }: { requestId: string; onApproved: (roomId: string) => void; onBack: () => void }) {
  const [status, setStatus] = useState<"pending" | "rejected">("pending");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const check = async () => {
      try {
        const next = await getGuestJoinStatus(requestId);
        if (!active) return;
        if (next.status === "approved") onApproved(next.roomId);
        else setStatus(next.status);
      } catch (cause) {
        if (active && cause instanceof ApiError && cause.status === 401) setError("This request needs the browser session that started it.");
      }
    };
    const initial = window.setTimeout(check, 0);
    const interval = window.setInterval(check, 2000);
    return () => { active = false; window.clearTimeout(initial); window.clearInterval(interval); };
  }, [onApproved, requestId]);

  const rejected = status === "rejected";
  return <main className="relative grid min-h-dvh place-items-center overflow-hidden bg-slate-950 bg-cover bg-center px-5 py-8 text-slate-950" style={{ backgroundImage: "linear-gradient(rgba(8, 24, 48, .28), rgba(8, 16, 29, .55)), url('/mountain-room-bg.png')" }}>
            <p className="text-center text-lg text-slate-500 items-center mt-20 font-serif absolute left-10 top-0">reso</p>
    <section className="h-[30rem] w-full max-w-sm rounded-md border border-white/65 bg-white/85 p-8 text-center shadow-2xl shadow-slate-950/35 backdrop-blur-xl sm:p-10">
      <p className="font-sans text-4xl">Reso</p>
      <p className="mt-7 text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">{rejected ? "Request declined" : "Request sent"}</p>
      <h1 className="mt-3 font-sans text-4xl leading-none">{rejected ? "Not this time." : "Waiting at the door."}</h1>
      <p className="mt-4 text-sm leading-6 text-slate-600">{rejected ? "The owner declined this request. Check the code and try again." : "The owner can approve you now. This page will let you in automatically."}</p>
      {error && <p className="mt-4 text-sm text-rose-600">{error}</p>}
      <button className="mt-7 h-12 w-full rounded-xl bg-slate-950 text-sm font-semibold text-white transition hover:bg-slate-800 active:translate-y-px" onClick={onBack}>{rejected ? "Try another code" : "Back to join"}</button>
    </section>
  </main>;
}
