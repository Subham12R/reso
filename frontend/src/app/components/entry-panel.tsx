"use client";

import { FormEvent, useState } from "react";
import { ApiError, createRoom, requestJoin } from "../lib/api";

type Props = {
  initialMode: "create" | "join";
  onCreated: (roomId: string, code: string) => void;
  onRequested: (requestId: string) => void;
};

export function EntryPanel({ initialMode, onCreated, onRequested }: Props) {
  const [mode, setMode] = useState(initialMode);
  const [displayName, setDisplayName] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = displayName.trim();
    if (!name || (mode === "join" && !code.trim())) {
      setError(mode === "join" ? "Add your name and room code." : "Add a display name first.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      if (mode === "create") {
        const room = await createRoom(name);
        onCreated(room.roomId, room.code);
      } else {
        const join = await requestJoin(code.trim(), name);
        onRequested(join.requestId);
      }
    } catch (cause) {
      setError(cause instanceof ApiError && cause.status === 404
        ? "That room is unavailable. Check the code and try again."
        : cause instanceof ApiError && cause.status === 409
          ? "All three room slots are in use. End an existing room, then try again."
          : "Ruse could not reach the room service. Try again in a moment.");
    } finally {
      setLoading(false);
    }
  }

  return <main className="relative grid min-h-dvh place-items-center overflow-hidden bg-slate-950 bg-cover bg-center px-5 py-8 text-slate-950" style={{ backgroundImage: "linear-gradient(rgba(8, 24, 48, .28), rgba(8, 16, 29, .55)), url('/mountain-room-bg.png')" }}>

    <div className="absolute inset-0 bg-gradient-to-b from-slate-950/10 via-transparent to-slate-950/55" />
    <section className="relative h-[35rem] w-full max-w-sm rounded-md border border-white/65 bg-white/82 p-7 shadow-2xl shadow-slate-950/35 backdrop-blur-xl sm:p-9">
      <div className="mb-7 text-center">
        <p className="mt-2 text-sm text-slate-600">Create a Watch Party. And Have the Best time!</p>
      </div>
      <div className="mb-6 grid grid-cols-2 rounded-2xl bg-slate-900/6 p-1" role="tablist" aria-label="Room action">
        {(["create", "join"] as const).map((value) => <button key={value} type="button" role="tab" aria-selected={mode === value} onClick={() => { setMode(value); setError(""); }} className={`min-h-11 rounded-xl px-3 text-sm font-medium transition ${mode === value ? "bg-white text-slate-950 shadow-sm" : "text-slate-500 hover:text-slate-950"}`}>
          {value === "create" ? "Create" : "Join"}
        </button>)}
      </div>
      <form className="grid gap-4" onSubmit={submit} noValidate>
        {mode === "join" && <label className="grid gap-1.5 text-sm font-medium text-slate-700"><span>Room code</span><input value={code} onChange={(event) => setCode(event.target.value)} placeholder="Paste the code" autoComplete="off" required aria-invalid={Boolean(error) || undefined} className="h-12 rounded-xl border border-slate-300 bg-white/80 px-4 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/25" /></label>}
        <label className="grid gap-1.5 text-sm font-medium text-slate-700"><span>Display name</span><input value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="What should friends call you?" autoComplete="name" maxLength={50} required aria-invalid={Boolean(error) || undefined} className="h-12 rounded-xl border border-slate-300 bg-white/80 px-4 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/25" /></label>
        <p className={`min-h-5 text-center text-xs ${error ? "text-rose-600" : "text-slate-500"}`} aria-live="polite">{error || "No account or permanent profile needed."}</p>
        <button className="h-12 rounded-xl cursor-pointer bg-zinc-900 border-2 border-zinc-900 shadow-md px-4 text-sm font-semibold text-white transition hover:bg-zinc-950 active:translate-y-px disabled:cursor-not-allowed disabled:opacity-60" disabled={loading}>{loading ? "Connecting..." : mode === "create" ? "Create private room" : "Request entry"}</button>
      </form>
  
    </section>
 
  </main>
}
