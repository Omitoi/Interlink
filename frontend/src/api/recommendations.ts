import api from "./axios";
import type { RecommendationsResponse, RecommendationsDetailedResponse } from "../types/api";
import type { CandidateDisplay, RecommendationWithScore } from "../types/ui";
import { hydrateCandidate } from "./users";

/** GET /recommendations → ordered list of user IDs (max 10) */
export async function getRecommendationIds(): Promise<number[]> {
  const { data } = await api.get<RecommendationsResponse>("/recommendations");
  return data.recommendations ?? [];
}

/** GET /recommendations/detailed → ordered list with scores and percentages */
export async function getRecommendationsWithScores(): Promise<RecommendationWithScore[]> {
  const { data } = await api.get<RecommendationsDetailedResponse>("/recommendations/detailed");
  return data.recommendations ?? [];
}

/**
 * Fetch the current recommendation IDs, hydrate each to a CandidateDisplay,
 * preserve the original order, and gracefully skip any racey 404s.
 */
export async function hydrateRecommendations(): Promise<CandidateDisplay[]> {
  const recommendations = await getRecommendationsWithScores();

  const results = await Promise.all(
    recommendations.map(async (rec) => {
      try {
        const candidate = await hydrateCandidate(rec.user_id);
        const enrichedCandidate: CandidateDisplay = {
          ...candidate,
          score: rec.score,
          score_percentage: rec.score_percentage,
          ...(rec.distance !== undefined && { distance: rec.distance }),
        };
        return enrichedCandidate;
      } catch (err: unknown) {
        // If the profile became inaccessible (e.g., perms changed) skip it.
        const error = err as { response?: { status?: number } };
        if (error?.response?.status === 404) return null;
        // Bubble up other errors (network, 500s)
        throw err;
      }
    })
  );

  // Keep order, drop nulls
  return results.filter((x): x is CandidateDisplay => x !== null);
}

export async function dismissCandidate(id: number): Promise<void> {
  await api.post(`/recommendations/${id}/dismiss`);
}

// POST /connections/{id}/request
type RequestResponse = {
  state: "pending" | "accepted" | "mismatch"
  connection_id?: number; // Not implemented yet on server side
}

export async function requestCandidate(id: number): Promise<RequestResponse> {
  try {
    const { data } = await api.post(`/connections/${id}/request`);
    const serverState = data?.state as "pending" | "accepted" | undefined;

    if (serverState === "pending" || serverState === "accepted") {
      return { state: serverState, connection_id: data?.connection_id };
    }
    // Unexpected successful payload shape → treat as mismatch
    return { state: "mismatch" }
  } catch (e: unknown) {
    // Map known "not actionable" outcomes to mismatch, bubble up true errors
    const error = e as { response?: { status?: number } };
    const code = error?.response?.status;
    if (code === 404 || code === 409) return { state: "mismatch" }
    throw e;
  }
  
}
