export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!response.ok) {
    const message = (await response.text()).trim();
    throw new ApiError(response.status, message || "Request failed");
  }
  return response.status === 204 ? (undefined as T) : response.json();
}

export type CreatedRoom = { roomId: string; code: string };
export type JoinRequest = { requestId: string; status: "pending" };
export type RoomState = { roomId: string; state: "active" | "ended"; expiresAt: string };
export type PendingRequest = { id: string; name: string; status: "pending" };
export type GuestJoinStatus = { status: "pending" | "approved" | "rejected"; roomId: string };
export type MediaAccess = { url: string; token: string; canPublish: boolean };

export const createRoom = (displayName: string) => request<CreatedRoom>("/api/v1/rooms", {
  method: "POST", body: JSON.stringify({ displayName }),
});
export const requestJoin = (code: string, displayName: string) => request<JoinRequest>("/api/v1/rooms/join-requests", {
  method: "POST", body: JSON.stringify({ code, displayName }),
});
export const getGuestJoinStatus = (requestId: string) => request<GuestJoinStatus>(`/api/v1/rooms/join-requests/${requestId}/status`);
export const getRoomState = (roomId: string) => request<RoomState>(`/api/v1/rooms/${roomId}/state`);
export const listPendingRequests = (roomId: string) => request<{ requests: PendingRequest[] }>(`/api/v1/rooms/${roomId}/join-requests`);
export const decideJoin = (roomId: string, requestId: string, decision: "approve" | "reject") => request<{ status: "approved" | "rejected" }>(`/api/v1/rooms/${roomId}/join-requests/${requestId}/${decision}`, { method: "POST" });
export const endRoom = (roomId: string) => request<{ state: "ended" }>(`/api/v1/rooms/${roomId}/end`, { method: "POST" });
export const getMediaAccess = (roomId: string, role?: "owner" | "guest") => request<MediaAccess>(`/api/v1/rooms/${roomId}/media/token`, { method: "POST", headers: role ? { "X-Ruse-Session-Role": role } : undefined });
