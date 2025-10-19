import api from "./axios";
import type {
  ConnectionRequestDisplay,
  ConnectionDisplay,
  UserSummary,
  AcceptConnectionResponse,
} from "../types/domain";


// --- Lightweight user cache for /users/:id ---------------------------------
const userCache = new Map<number, UserSummary>();

async function fetchUserSummary(userId: number): Promise<UserSummary> {
  const cached = userCache.get(userId);
  if (cached) return cached;

  try {
    const { data } = await api.get<{ id: number; display_name: string; profile_picture?: string | null; is_online: boolean }>(
      `/users/${userId}`
    );
    const u: UserSummary = {
      id: data.id,
      display_name: data.display_name,
      profile_picture: data.profile_picture ?? null,
      is_online: data.is_online
    };
    userCache.set(userId, u);
    return u;
  } catch (err: unknown) {
    // If the backend denies access (or if the ID is not a valid user ID), provide a placeholder instead of breaking the UI.
    const error = err as { response?: { status?: number } };
    if (error?.response?.status === 404) {
      console.warn(`[connections] /users/${userId} returned 404 - using placeholder`);
      const u: UserSummary = { id: userId, display_name: `User #${userId}`, profile_picture: null };
      userCache.set(userId, u);
      return u;
    }
    throw err;
  }
}

/**
 * GET pending requests sent TO the current user.
 */
export async function fetchConnectionRequests(): Promise<ConnectionRequestDisplay[]> {
  try {
    const res = await api.get<number[] | { from_user_id: number }[] | { requests: number[] | { from_user_id: number }[] }>("/connections/requests"); // or .../incoming depending on backend implementation
    // Accepts formats: number[], { from_user_id: number }[], or { requests: ... }
    const raw = Array.isArray(res.data) ? res.data : (res.data?.requests ?? []);
    const peerIds: number[] = (raw as (number | { from_user_id: number })[])
      .map(x => (typeof x === "number" ? x : x?.from_user_id))
      .filter((v: unknown): v is number => Number.isInteger(v));

    if (peerIds.length === 0) return [];

    const items = await Promise.all(
      peerIds.map(async (peerId) => {
        const user = await fetchUserSummary(peerId); // placeholder-safe version
        return { id: peerId, user } as ConnectionRequestDisplay;
      })
    );

    return items;
  } catch (err) {
    // During development the backend may return 500 – do not break the UI.
    console.warn("[connections] fetchConnectionRequests failed → returning []", err);
    return [];
  }
}

/**
 * GET /connections → { connections: number[] }
 * where numbers are peer user IDs
 */
export async function fetchConnections(): Promise<ConnectionDisplay[]> {
  const res = await api.get<{ connections: number[] }>("/connections");
  const ids = res.data.connections ?? [];
  const items = await Promise.all(
    ids.map(async (peerId) => {
      const user = await fetchUserSummary(peerId);
      // Use peerId as the "connection id" in the frontend
      const display: ConnectionDisplay = {
        id: peerId,
        peer_user_id: peerId,
        created_at: "", // not available from current endpoint
        user,
        last_message_snippet: null,
        last_message_at: null,
      };
      return display;
    })
  );
  return items;
}

/**
 * POST /connections/{userId}/accept → { state, connection_id? }
 */
export async function acceptConnection(userId: number): Promise<AcceptConnectionResponse> {
  const res = await api.post<AcceptConnectionResponse>(`/connections/${userId}/accept`);
  // If backend does not always return connection_id, normalize to a safe value
  return { connection_id: res.data?.connection_id ?? userId };
}

/**
 * POST /connections/{userId}/decline
 */
export async function declineConnection(userId: number): Promise<void> {
  await api.post(`/connections/${userId}/decline`);
}

/**
 * DELETE /connections/{userId}
 */
export async function deleteConnection(userId: number): Promise<void> {
  await api.delete(`/connections/${userId}`);
}
