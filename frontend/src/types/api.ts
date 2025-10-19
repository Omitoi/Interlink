// Shapes mirror the backend's JSON (snake_case keys).

export type UserPublic = {
  id: number;
  display_name: string;        // from GET /users/{id} or /me
  profile_picture?: string | null | undefined;     // '' if none
};

export type ProfileOverview = {
  id: number;
  display_name: string;
  profile_picture: string;
  about_me: string;
  is_online: boolean;
  location_lat?: number;
  location_lon?: number;
};

export type RecommendationsResponse = {
  recommendations: number[];   // max 10 ids, strongest-first
};

export type RecommendationsDetailedResponse = {
  recommendations: {
    user_id: number;
    score: number;
    score_percentage: number;
    distance?: number;
  }[];
};

export type ConnectionsResponse = {
  connections: number[];       // accepted connections (ids)
};

export type UserBiography = {
    id: number;
    analog_passions: string[];
    digital_delights: string[];
    collaboration_interests: string;
    favorite_food: string;
    favorite_music: string;
};

export type MeProfileResponse = {
  id: number;
  display_name: string;
  profile_picture: string;
  about_me: string;
  location_lat?: number;
  location_lon?: number;
  max_radius_km?: number;
};
