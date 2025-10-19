import api from "./axios";
import { ChatMessage } from "@/types/chat";

export async function fetchChatHistory(
  otherUserId: number,
  opts?: { limit?: number; before?: string }
): Promise<ChatMessage[]> {
  const params = new URLSearchParams();
  if (opts?.limit) params.set("limit", String(opts.limit));
  if (opts?.before) params.set("before", opts.before);
  const { data } = await api.get<ChatMessage[]>(
    `/chats/${otherUserId}/messages?${params.toString()}`
  );
  return data;
}
