export function openChatSocket(token: string): WebSocket {
  // Get the API base URL from environment variable, same as axios config
  const baseURL = 
    (import.meta.env.VITE_API_BASE_URL as string | undefined)?.replace(/\/+$/, "") ||
    "http://localhost:8080";
  
  // Convert HTTP(S) URL to WebSocket URL
  const wsURL = baseURL.replace(/^https?:\/\//, (match) => {
    return match === 'https://' ? 'wss://' : 'ws://';
  });
  
  const url = `${wsURL}/ws/chat?token=${encodeURIComponent(token)}`;
  return new WebSocket(url);
}

// PAUNO 12.8.25: Browsers cannot set arbitrary headers (like Authorization) on WebSocket
// The workaround here is to have query param: 'ws://.../ws/chat?token=...' and read
// it server-side (see wsChatHandler in backend/chat.go).