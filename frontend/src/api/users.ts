import api from "./axios";
import type { UserPublic, ProfileOverview } from "../types/api";
import type { CandidateDisplay } from "../types/ui";

/** GET /users/{id} — public shell */
export async function getUserPublic(id: number): Promise<UserPublic> {
  const { data } = await api.get<UserPublic>(`/users/${id}`);
  return data;
}

/** GET /users/{id}/profile — limited profile (allowed for recommendations/pending/accepted) */
export async function getUserProfile(id: number): Promise<ProfileOverview> {
  const { data } = await api.get<ProfileOverview>(`/users/${id}/profile`);
  return data;
}

/** Fetch both public+profile and merge into a CandidateDisplay */
export async function hydrateCandidate(id: number): Promise<CandidateDisplay> {
  const [u, p] = await Promise.all([getUserPublic(id), getUserProfile(id)]);
  return {
    id: u.id,
    display_name: u.display_name,
    profile_picture: u.profile_picture,
    is_online: p.is_online,
    about_me: p.about_me,
  };
}
