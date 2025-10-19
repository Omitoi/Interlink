// src/types/chat.ts
export type ChatMessage = {
  type: "message";
  chat_id: number;
  from: number;
  to?: number;
  body: string;
  ts: string; // ISO
};

export type ServerEvent =
  | { type: "message"; from?: number; data?: ChatMessage }
  | { type: "typing"; from?: number; data?: { typing: boolean } }
  | { type: "info"; from?: number; data?: string }
  | { type: "error"; from?: number; data?: string };
