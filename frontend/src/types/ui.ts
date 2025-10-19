/**
 * CandidateDisplay = the minimal data we show in the Recommendations list
 * after hydrating an id with GET /users/{id} AND GET /users/{id}/profile.
 */
export type CandidateDisplay = {
  id: number;
  display_name: string;
  profile_picture?: string | null | undefined;
  is_online: boolean;
  about_me?: string | null;      // from ProfileOverview
  score?: number;               // raw compatibility score
  score_percentage?: number;    // compatibility as percentage
  distance?: number;            // distance in meters
};

/**
 * RecommendationWithScore = backend response for detailed recommendations
 */
export type RecommendationWithScore = {
  user_id: number;
  score: number;
  score_percentage: number;
  distance?: number;
};

