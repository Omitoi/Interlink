import { useCallback, useEffect, useRef, useState } from "react";
import type { CandidateDisplay } from "../types/ui";
import { hydrateRecommendations } from "../api/recommendations";

export type UseRecommendationsResult = {
  loading: boolean;                 // true on initial load
  refreshing: boolean;              // true on manual refetch
  error: string | null;             // human-readable error, if any
  candidates: CandidateDisplay[];   // hydrated, ordered by backend score
  refetch: () => void;              // manual reload
};

/*
loading vs refreshing: lets you show a full-page skeleton for the first load 
and a small spinner for manual refreshes without flashing the whole page.
*/

export default function useRecommendations(): UseRecommendationsResult {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<CandidateDisplay[]>([]);

  // Guards against stale setState when multiple loads overlap or component unmounts.
  const tick = useRef(0);

  const load = useCallback(async (mode: "initial" | "refresh" = "initial") => {
    const myTick = ++tick.current;
    if (mode === "initial") setLoading(true);
    if (mode === "refresh") setRefreshing(true);
    setError(null);

    try {
      const list = await hydrateRecommendations();
      if (myTick !== tick.current) return; // stale result, ignore
      setCandidates(list);
    } catch (err: unknown) {
      if (myTick !== tick.current) return;
      const error = err as { response?: { data?: { message?: string } }; message?: string };
      const msg =
        error?.response?.data?.message ||
        error?.message ||
        "Failed to load recommendations.";
      setError(msg);
    } finally {
      if (myTick === tick.current) {
        if (mode === "initial") setLoading(false);
        if (mode === "refresh") setRefreshing(false);
      }
    }
  }, []);

  // Initial load
  useEffect(() => {
    load("initial");
    // On unmount, bump tick so any late promises are ignored.
    return () => {
      tick.current = tick.current + 1;
    };
  }, [load]);

  const refetch = useCallback(() => load("refresh"), [load]);

  return { loading, refreshing, error, candidates, refetch };
}
