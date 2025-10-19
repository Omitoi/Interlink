// src/types/domain.ts

// --- Basic aliases ----------------------------------------------------------
export type ID = number;
export type ISODateTime = string; // e.g. "2025-09-01T10:15:00Z"

// --- Compact user representation for cards ---------------------------------
export type UserSummary = {
  id: ID;
  display_name: string;
  profile_picture?: string | null; // path or URL
  is_online?: boolean;
  // extensibility: last_seen_at?: ISODateTime; status?: "online" | "offline"
};

// --- Connection requests (incoming requests) -------------------------------

export type ConnectionRequest = {
  id: ID;
  created_at?: ISODateTime;
  // possible duplicate scenario (if already accepted from the other side):
  status?: "pending" | "accepted" | "declined";
  // if backend already includes the user, you can use it directly:
  user?: UserSummary;
};

// For UI: guaranteed to have a user for display purposes
export type ConnectionRequestDisplay = ConnectionRequest & {
  user: UserSummary;
};

// --- Connections (active connections) --------------------------------------
/**
 * NOTE: match these keys with your backend schema:
 * - id: connection id
 * - peer_user_id: the other party’s user id
 */
export type Connection = {
  id: ID;
  peer_user_id: ID;
  created_at: ISODateTime;
  // quick list metadata:
  last_message_snippet?: string | null;
  last_message_at?: ISODateTime | null;
  // if backend already includes the user:
  user?: UserSummary;
};

export type ConnectionDisplay = Connection & {
  user: UserSummary; // required in UI
};

// --- API responses (if needed) ---------------------------------------------
export type AcceptConnectionResponse = {
  connection_id: ID;
};

// Generic error payload (extend to match your backend)
export type ApiErrorPayload = {
  error?: string;
  message?: string;
  code?: string | number;
};

// --- Utility types (optional) ----------------------------------------------
export type BusyAction = "accept" | "decline" | "remove";
export type BusyMap = Record<ID, BusyAction | undefined>;
