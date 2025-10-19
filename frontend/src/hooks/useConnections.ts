// src/hooks/useConnections.ts
// -----------------------------------------------------------------------------
// Hook for providing both the Connections list and the Requests list
// to the Dashboard via a single, unified hook. This also demonstrates:
//
//  1) State handling (loading, refreshing, error)
//  2) Busy states for individual rows (accept/decline/remove)
//  3) Optimistic updates so the UI reacts immediately
//
// -----------------------------------------------------------------------------


import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type BusyMap,
  type ConnectionDisplay,
  type ConnectionRequestDisplay,
} from "../types/domain";
import {
  fetchConnections,
  fetchConnectionRequests,
  acceptConnection as apiAccept,
  declineConnection as apiDecline,
  deleteConnection as apiDelete,
} from "../api/connections";

// -----------------------------------------------------------------------------
// 0) Return type – so you can see at a glance what the hook provides
// -----------------------------------------------------------------------------
export type UseConnectionsReturn = {
  // Global UI state
  loading: boolean;     // true while the initial fetch is in progress
  refreshing: boolean;  // true while a manual refetch is in progress
  error: string | null; // latest error message, if any

  // Data lists
  connections: ConnectionDisplay[];
  requests: ConnectionRequestDisplay[];

  // Busy states for cards (key = peer user ID)
  busy: BusyMap; // "accept" | "decline" | "remove" | undefined

  // Actions (called from buttons on cards)
  refetch: () => Promise<void>;
  accept: (userId: number) => Promise<void>;   // accept a pending request
  decline: (userId: number) => Promise<void>;  // decline a pending request
  remove: (userId: number) => Promise<void>;   // remove an existing connection
};

// -----------------------------------------------------------------------------
// 1) Error handling helper – converts unknown error -> clean string
// -----------------------------------------------------------------------------
function toErrorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err && typeof (err as { message: unknown }).message === "string") {
    return (err as { message: string }).message;
  }
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
}

// -----------------------------------------------------------------------------
// 2) The actual hook
// -----------------------------------------------------------------------------
export default function useConnections(): UseConnectionsReturn {
  // 2.1) Base state: loading, refreshing, error
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 2.2) Data lists: connections and requests
  const [connections, setConnections] = useState<ConnectionDisplay[]>([]);
  const [requests, setRequests] = useState<ConnectionRequestDisplay[]>([]);

  // 2.3) Busy state: which row is currently processing an API call
  //      Key = peer user ID. Note: our API uses PEER USER ID in all operations.
  //      (accept/decline/remove). ConnectionDisplay.id = peerId, RequestDisplay.id = peerId.
  const [busy, setBusy] = useState<BusyMap>({});

  // ---------------------------------------------------------------------------
  // 2.4) Data load – fetch both lists in parallel with Promise.all
  // ---------------------------------------------------------------------------
  const load = useCallback(async () => {
    setError(null);

    const [reqs, conns] = await Promise.allSettled([
      fetchConnectionRequests(),
      fetchConnections(),
    ]);

    if (reqs.status === "fulfilled") {
      setRequests(reqs.value);
    } else {
      console.warn("[useConnections] request list failed", reqs.reason);
      setRequests([]); // keep UI alive
    }

    if (conns.status === "fulfilled") {
      setConnections(conns.value);
    } else {
      console.warn("[useConnections] connections list failed", conns.reason);
      setConnections([]);
    }
  }, []);

  // 2.5) Initial load on component mount – run exactly once
  useEffect(() => {
    let alive = true; // cancellation flag if component unmounts mid-fetch
    (async () => {
      try {
        setLoading(true);
        await load();
      } catch (e) {
        if (alive) setError(toErrorMessage(e));
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [load]);

  // 2.6) Manual refetch – same as above but without the initial-load flag
  const refetch = useCallback(async () => {
    try {
      setRefreshing(true);
      await load();
    } catch (e) {
      setError(toErrorMessage(e));
    } finally {
      setRefreshing(false);
    }
  }, [load]);

  // ---------------------------------------------------------------------------
  // 2.7) Actions: accept, decline, remove
  //      Common pattern:
  //        a) busy[userId] = "..."
  //        b) call API
  //        c) optimistic update to lists (and/or refetch)
  //        d) clear busy state
  // ---------------------------------------------------------------------------

  const accept = useCallback(async (userId: number) => {
    setBusy((m) => ({ ...m, [userId]: "accept" }));
    try {
      // 1) Server accepts the request
      const { connection_id } = await apiAccept(userId);

      // 2) Optimistic update: remove from request list immediately
      setRequests((list) => list.filter((r) => r.id !== userId));

      // 3) Optimistic addition to connections list
      setConnections((list) => {
        if (list.some((c) => c.id === userId)) return list;

        const req = requests.find((r) => r.id === userId);
        const user = req?.user;

        if (!user) {
          // If request not found, trigger a refetch as a safety net
          void refetch();
          return list;
        }

        const optimistic: ConnectionDisplay = {
          id: connection_id ?? userId,   // in frontend we keep id === peerId
          peer_user_id: userId,
          created_at: "",                // no info from current endpoint
          user,
          last_message_snippet: null,
          last_message_at: null,
        };
        return [optimistic, ...list];
      });
    } catch (e) {
      setError(toErrorMessage(e));
    } finally {
      setBusy((m) => ({ ...m, [userId]: undefined }));
    }
  }, [requests, refetch]);

  const decline = useCallback(async (userId: number) => {
    setBusy((m) => ({ ...m, [userId]: "decline" }));
    try {
      await apiDecline(userId);
      // Remove from request list – no longer pending
      setRequests((list) => list.filter((r) => r.id !== userId));
    } catch (e) {
      setError(toErrorMessage(e));
    } finally {
      setBusy((m) => ({ ...m, [userId]: undefined }));
    }
  }, []);

  const remove = useCallback(async (userId: number) => {
    setBusy((m) => ({ ...m, [userId]: "remove" }));
    try {
      await apiDelete(userId);
      // Remove from connections list – our id === peerId
      setConnections((list) => list.filter((c) => c.id !== userId));
    } catch (e) {
      setError(toErrorMessage(e));
    } finally {
      setBusy((m) => ({ ...m, [userId]: undefined }));
    }
  }, []);

  // ---------------------------------------------------------------------------
  // 2.8) Derived values (optional): e.g. sort lists by name
  // ---------------------------------------------------------------------------
  const orderedConnections = useMemo(() => {
    return [...connections].sort((a, b) => {
      const A = a.user.display_name.toLowerCase();
      const B = b.user.display_name.toLowerCase();
      return A.localeCompare(B);
    });
  }, [connections]);

  const orderedRequests = useMemo(() => {
    return [...requests].sort((a, b) => {
      const A = a.user.display_name.toLowerCase();
      const B = b.user.display_name.toLowerCase();
      return A.localeCompare(B);
    });
  }, [requests]);

  // 2.9) Return the API
  return {
    loading,
    refreshing,
    error,
    connections: orderedConnections,
    requests: orderedRequests,
    busy,
    refetch,
    accept,
    decline,
    remove,
  };
}
