import React, { useEffect, useMemo, useRef, useState, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import useMyProfile from "../hooks/useMyProfile";
import { getUserProfile } from "../api/users";
import { calculateDistanceIfAvailable } from "../utils/distance";
import api from "../api/axios";
import { openChatSocket } from "../api/ws";
import { fetchChatHistory } from "../api/chat";
import type { ChatMessage, ServerEvent } from "../types/chat";
import type { ProfileOverview } from "../types/api";
import s from "./Chat.module.css";
import { History, Send, ArrowLeft } from 'lucide-react'
import Avatar from "../components/Avatar"
import ProfileModal from "../components/ProfileModal"

const PAGE_SIZE = 50

/**
 * --- TYPES USED BY THE UI ---
 */

// Server pushes ChatMessage over WS.
// We'll map it to our local UI Message for the list.
type Message = {
  id: string;
  from: number;
  to: number;
  body: string;
  at: number;
};

type Conversation = {
  peerId: number;
  messages: Message[];
  peerTyping: boolean;
  hasMore?: boolean; // Are there older messages on the server?
  oldestAt?: number; // Timestamp (ms) of the oldest message in this UI buffer
};

/**
 * The summary shape from GET /chats/summary .
 * One row per peer (the other person), already scoped to the logged-in user.
 */
type ChatPeerSummary = {
  userId: number;
  userName: string;
  profilePicture?: string | null;
  lastMessageAt?: string | null; // RFC3339 from Go (time.Time). Converted to ms when needed.
  unreadMessages: number;        // integer; 0 means no badge
  isOnline: boolean;
};

/** Format timestamps for message bubbles */
function formatTime(ts: number) {
  const d = new Date(ts);
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

/** Convert ISO-ish input (or undefined) to ms number or 0 */
function toMs(ts?: string | null): number {
  return ts ? Date.parse(ts) : 0;
}

const Chat: React.FC = () => {
  const { token, user } = useAuth();
  const { profile: myProfile } = useMyProfile();
  const myId = user?.id;

  const { peerId } = useParams<{ peerId?: string }>();
  const navigate = useNavigate();

  // ProfileModal state for showing user profiles when clicking avatars
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [selectedUserProfile, setSelectedUserProfile] = useState<ProfileOverview | null>(null);

  // Coalesce multiple read marks per peer
  const readInFlight = useRef<Set<number>>(new Set());

  const markRead = useCallback((peer: number) => {
    if (readInFlight.current.has(peer)) return;
    readInFlight.current.add(peer);
    api.post("/chats/read", null, { params: { peer_id: peer } })
      .finally(() => { readInFlight.current.delete(peer); });
  }, []);

  // Fetch the selected user's profile when selectedUserId changes
  useEffect(() => {
    if (selectedUserId) {
      getUserProfile(selectedUserId)
        .then(setSelectedUserProfile)
        .catch(() => setSelectedUserProfile(null));
    } else {
      // Reset profile asynchronously to avoid direct setState in effect
      const timer = setTimeout(() => setSelectedUserProfile(null), 0);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [selectedUserId]);

  // Calculate distance if both user locations are available
  const distance = useMemo(() => {
    if (!myProfile || !selectedUserProfile) return undefined;
    return calculateDistanceIfAvailable(
      myProfile.location_lat,
      myProfile.location_lon,
      selectedUserProfile.location_lat,
      selectedUserProfile.location_lon
    );
  }, [myProfile, selectedUserProfile]);

  // ProfileModal handlers
  const handleOpenProfile = useCallback((userId: number) => {
    setSelectedUserId(userId);
  }, []);

  const handleCloseProfile = useCallback(() => {
    setSelectedUserId(null);
  }, []);


  // --- WebSocket management ---
  // useRef based socket management. Avoids unnecessary opening and closing of socket on state change.
  const socketRef = useRef<WebSocket | null>(null);

  // Keep the freshest values here so onmessage sees current IDs
  const myIdRef = useRef<number | null>(null);
  const activePeerRef = useRef<number | null>(null);

  // Update refs when values change
  useEffect(() => {
    myIdRef.current = myId ?? null;
  }, [myId]);

  // A small queue for messages the user tries to send before the socket opens
  const outboxRef = useRef<string[]>([]);

  // --- LEFT PANE: list of peers from /chats/summary ---
  // We keep the raw array and sort it on render by lastMessageAt (desc).
  const [peers, setPeers] = useState<ChatPeerSummary[]>([]);

  // Active peer (right pane). We keep a ref in sync so WS handlers see the current value.
  const [activePeer, setActivePeer] = useState<number | null>(null);

  // Update activePeerRef when activePeer changes
  useEffect(() => {
    activePeerRef.current = activePeer ?? null;
  }, [activePeer]);

  // Per-peer conversation buffers (messages already mapped to UI Message shape).
  const [convos, setConvos] = useState<Record<number, Conversation>>({});

  // Composer
  const [draft, setDraft] = useState("");

  // Scrolling + typing
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingDebounceRef = useRef<number | null>(null);

  // Load-older control
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [showLoadOlderButton, setShowLoadOlderButton] = useState(false);

  // Mobile view state
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 1024);
  const [showMobileChatView, setShowMobileChatView] = useState(false);

  // Scroll detection for "Load older" button
  const handleScroll = useCallback(() => {
    const container = messagesEndRef.current;
    if (!container) return;

    // Show button when user is within 100px of the top
    const isNearTop = container.scrollTop <= 100;
    setShowLoadOlderButton(isNearTop);
  }, []);

  // Add scroll listener
  useEffect(() => {
    const container = messagesEndRef.current;
    if (!container) return;

    container.addEventListener('scroll', handleScroll);
    // Check initial position
    handleScroll();

    return () => {
      container.removeEventListener('scroll', handleScroll);
    };
  }, [handleScroll, activePeer]);

  // Handle back navigation on mobile
  const handleBackToList = useCallback(() => {
    setActivePeer(null);
    setShowMobileChatView(false);
    navigate('/chat', { replace: true });
  }, [navigate]);

  // Handle window resize for mobile detection
  useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth <= 1024;
      setIsMobile(mobile);
      // If switching to desktop and mobile chat view is open, show both panels
      if (!mobile && showMobileChatView) {
        setShowMobileChatView(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [showMobileChatView]);

  // React to URL param changes: /chat/:peerId?
  useEffect(() => {
    if (!peerId) {
      // If no peerId and on mobile, make sure we're showing chat list
      if (isMobile) {
        const timer = setTimeout(() => setShowMobileChatView(false), 0);
        return () => clearTimeout(timer);
      }
      return undefined;
    }
    const idNum = Number(peerId);
    if (!Number.isNaN(idNum)) {
      // Set active peer asynchronously to avoid direct setState in effect
      const timer = setTimeout(() => {
        setActivePeer(idNum);
        
        // On mobile, show chat view when a specific chat is selected
        if (isMobile) {
          setShowMobileChatView(true);
        }

        // Save the latest so that /chat without any params can open it
        try { 
          localStorage.setItem("lastPeerId", String(idNum)); 
        } catch (error) {
          console.warn("Failed to save lastPeerId to localStorage:", error);
        }
      }, 0);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [peerId, isMobile]);

  /**
   * Load older messages for the active peer (top pagination).
   * - Uses fetchChatHistory(limit, before)
   * - After append, scrolls to top (so "Load older" takes you upwards)
   */
  const loadOlder = useCallback(async () => {
    if (activePeer == null || loadingOlder) return;
    const convo = convos[activePeer];
    if (!convo || !convo.hasMore || !convo.oldestAt) return;

    setLoadingOlder(true);
    const beforeISO = new Date(convo.oldestAt).toISOString();

    try {
      const desc = await fetchChatHistory(activePeer, { limit: PAGE_SIZE, before: beforeISO });
      const ascOlder: Message[] = [...desc].reverse().map((m: ChatMessage) => ({
        id: crypto.randomUUID(),
        from: m.from,
        to: m.to ?? activePeer,
        body: m.body,
        at: Date.parse(m.ts),
      }));

      setConvos(prev => {
        const cur: Conversation = prev[activePeer] ?? {
          peerId: activePeer,
          messages: [],
          peerTyping: false,
        };

        const newMsgs = [...ascOlder, ...cur.messages];
        const oldest = ascOlder[0]?.at; 

        // NB! oldestAt must be defined. So has to be checked separately.
        const next: Conversation = {
          ...cur,
          messages: newMsgs,
          peerTyping: false,
          hasMore: desc.length === PAGE_SIZE,
          ...(oldest !== undefined ? { oldestAt: oldest } : {}),
        };

        return { ...prev, [activePeer]: next };
      });


      // wait for render and go to top
      requestAnimationFrame(() => {
        const el = messagesEndRef.current;
        if (el) {
          el.scrollTo({ top: 0, behavior: "smooth" });
        }
        setLoadingOlder(false);
      });
    } catch {
      setLoadingOlder(false);
    }
  }, [activePeer, convos, loadingOlder]);

  /**
   * Load the sidebar peers with ONE request.
   *
   * Steps:
   * 1) GET /chats/summary to fetch peers with { userId, userName, picture, lastMessageAt, unread }
   * 2) Choose default activePeer (url param wins; otherwise first item in sorted list)
   */
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const res = await api.get<ChatPeerSummary[]>("/chats/summary");
        if (!alive) return;

        const list = Array.isArray(res.data) ? res.data : [];
        setPeers(list);

        // If no explicit peer in URL and no activePeer yet, default to top item by recency.
        const hasParam = typeof peerId === "string" && peerId.length > 0;
        if (!hasParam && (activePeer == null) && list.length > 0) {
          // sort by lastMessageAt desc (missing timestamps sink)
          const sorted = [...list].sort((a, b) => toMs(b.lastMessageAt) - toMs(a.lastMessageAt));
          const first = sorted[0]?.userId;
          if (first != null) {
            setActivePeer(first);
            try { 
              localStorage.setItem("lastPeerId", String(first)); 
            } catch (error) {
              console.warn("Failed to save lastPeerId to localStorage:", error);
            }
          }
        }
      } catch {
        // ignore failures; empty peers list will show "No connections yet."
      }
    })();
    return () => { alive = false; };
  }, [peerId, activePeer]);

  /**
   * When activePeer changes:
   * - Fetch newest PAGE_SIZE messages (DESC on server, reversed to ASC here)
   * - Mark read on server (getChatHistoryHandler does this)
   * - Locally zero the unread badge for that peer (so UI reacts instantly)
   */
  useEffect(() => {
    if (activePeer == null) return;
    let alive = true;

    (async () => {
      try {
        const desc = await fetchChatHistory(activePeer, { limit: PAGE_SIZE });
        if (!alive) return;

        const asc: Message[] = [...desc].reverse().map((m: ChatMessage) => ({
          id: crypto.randomUUID(),
          from: m.from,
          to: m.to ?? activePeer,
          body: m.body,
          at: Date.parse(m.ts),
        }));

        setConvos(prev => {
          const base: Conversation = {
            peerId: activePeer,
            messages: asc,
            peerTyping: false,
            hasMore: desc.length === PAGE_SIZE,
            ...(asc[0]?.at !== undefined ? { oldestAt: asc[0].at } : {}),
          };
          return { ...prev, [activePeer]: base };
        });

        // Locally clear unread for this peer (server already marked read via handler).
        setPeers(prev =>
          prev.map(p => (p.userId === activePeer ? { ...p, unreadMessages: 0 } : p))
        );
      } catch {
        // Silently ignore errors during message loading
      }
    })();

    return () => { alive = false; };
  }, [activePeer]);

  // --- WebSocket helpers ---

  // Clear outbox when the socket is open
  function flushOutbox() {
    const s = socketRef.current;
    if (!s || s.readyState !== WebSocket.OPEN) return;
    while (outboxRef.current.length) {
      const payload = outboxRef.current.shift()!;
      try { s.send(payload); } catch { break; }
    }
  }

  // Fix all the socket's event handlers to one location
  const attachSocketHandlers = useCallback((socket: WebSocket) => {
    socket.onopen = () => {
      flushOutbox();
    };

    socket.onmessage = (evt) => {
      try {
        const ev: ServerEvent = JSON.parse(evt.data);
        if (ev.type === "message" && myIdRef.current != null) {
          const m = ev.data as ChatMessage;
          const myIdNow = myIdRef.current!;
          // Which conversation bucket does this belong to?
          const peer = m.from === myIdNow ? (m.to ?? activePeerRef.current ?? -1) : m.from;
          const msg: Message = {
            id: crypto.randomUUID(),
            from: m.from,
            to: peer,
            body: m.body,
            at: Date.parse(m.ts),
          };

          // 1) Append message to that conversation
          setConvos((prev) => {
            const c = prev[peer] ?? { peerId: peer, messages: [], peerTyping: false };
            return { ...prev, [peer]: { ...c, messages: [...c.messages, msg], peerTyping: false } };
          });

          // 2) Update sidebar: bump lastMessageAt, and unread++ if incoming & not currently open
          setPeers(prev => {
    const incoming = m.from !== myIdNow;
            const isActive = activePeerRef.current === peer;
            return prev.map(p => {
              if (p.userId !== peer) return p;
              const nowIso = new Date(msg.at).toISOString();
              const nextUnread =
                incoming && !isActive ? (p.unreadMessages || 0) + 1 : p.unreadMessages || 0;
              return { ...p, lastMessageAt: nowIso, unreadMessages: nextUnread };
            });
          });

          // If I *just saw* the incoming message in the active thread, tell the server it's read.
          if (m.from !== myIdNow && activePeerRef.current === peer) {
            markRead(peer);
          }

        } else if (ev.type === "typing") {
          const from = ev.from;
          if (from != null) {
            setConvos((prev) => {
              const c = prev[from] ?? { peerId: from, messages: [], peerTyping: false };
              return { ...prev, [from]: { ...c, peerTyping: true } };
            });
            // clear after a short delay
            window.setTimeout(() => {
              setConvos((prev) => {
                const c = prev[from];
                if (!c) return prev;
                return { ...prev, [from]: { ...c, peerTyping: false } };
              });
            }, 1200);
          }
        }
      } catch {
        // ignore malformed
      }
    };

    socket.onclose = () => {
      // Socket closed (e.g. server restart). No reconnect in MVP.
      // The user can send a message, which will be added to the queue and sent when a new socket is opened
      // (when the user loads the pages or a reconnect is added)

    };
    socket.onerror = () => {
      // No special error handling in MVP.
    };
  }, [markRead]);

  // Open socket once when token & myId exist
  useEffect(() => {
    if (!token || myId == null) return;
    const socket = openChatSocket(token);
    socketRef.current = socket;
    attachSocketHandlers(socket);
    return () => {
      try { 
        socket.close(); 
      } catch (error) {
        console.warn("Failed to close WebSocket:", error);
      }
      if (socketRef.current === socket) socketRef.current = null;
    };
    // NB! effect does not depend on activePeer -> socket won't open/close on peer change
  }, [token, myId, attachSocketHandlers]);

  // Auto-scroll bottom on new messages for the active peer (except when loading older)
  const activeMessagesLength = activePeer != null ? convos[activePeer]?.messages.length : 0;
  useEffect(() => {
    if (loadingOlder) return;
    messagesEndRef.current?.scrollTo({ top: messagesEndRef.current.scrollHeight, behavior: "smooth" });
  }, [activePeer, activeMessagesLength, loadingOlder]);

  const activeConvo = useMemo(
    () => (activePeer != null ? convos[activePeer] : undefined),
    [activePeer, convos]
  );

  // 'Can it be sent?' is checked according to the REAL state of the socket.
  // In addition: if the socket is not open, the message is added to outbox and won't be lost
  const canSend = useMemo(
    () => myId != null && activePeer != null && draft.trim().length > 0,
    [myId, activePeer, draft]
  );

  const sendMessage = () => {
    if (myId == null || activePeer == null) return;
    const body = draft.trim();
    if (!body) return;
    const payload = JSON.stringify({ type: "message", to: activePeer, body });

    // If socket is open, send immediately. Otherwise add to outbox to wait for onopen.
    if (socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(payload);
    } else {
      outboxRef.current.push(payload);
    }

    setDraft("");
    // Note: we don't preemptively zero unread here; server will echo back,
    // and onmessage handler will bump lastMessageAt for this peer.
  };

  const sendTyping = () => {
    if (myId == null || activePeer == null) return;
    if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) return;

    // basic debounce: at most once per 500ms
    if (typingDebounceRef.current) return;
    typingDebounceRef.current = window.setTimeout(() => {
      if (typingDebounceRef.current) {
        window.clearTimeout(typingDebounceRef.current);
      }
      typingDebounceRef.current = null;
    }, 500) as unknown as number;
    
    socketRef.current.send(JSON.stringify({ type: "typing", to: activePeer }));
  };

  /**
   * Small helpers for sidebar and avatar image rendering
   */
  const sortedPeers = useMemo(() => {
    // Sort by lastMessageAt desc (undefined last -> bottom)
    return [...peers].sort((a, b) => toMs(b.lastMessageAt) - toMs(a.lastMessageAt));
  }, [peers]);

  const nameFor = (peerUserId: number) =>
    peers.find(p => p.userId === peerUserId)?.userName ?? `User ${peerUserId}`;

  const activePeerSummary = useMemo(
    () => (activePeer != null ? peers.find(p => p.userId === activePeer) : undefined),
    [activePeer, peers]
  );

  return (
    <div className={`${s.layout} ${isMobile ? s.mobileLayout : ''}`}>
      {/* Sidebar - on mobile, hidden when showing individual chat */}
      <aside className={`${s.sidebar} ${isMobile && showMobileChatView ? s.hiddenOnMobile : ''}`}>
        <div className={s.sidebarHeader}>Connections</div>
        {sortedPeers.length === 0 && <div className={s.sub}>No connections yet.</div>}

        <div className={s.recipientList}>
           {sortedPeers.map((p) => {
            const id = p.userId;
            const isActive = activePeer === id;

            // Show a short preview text from local buffer if we have it; otherwise lastMessageAt time.
            const lastMsg = convos[id]?.messages?.[convos[id]?.messages.length - 1];
            const preview = lastMsg
              ? `Last: ${lastMsg.body.slice(0, 24)}${lastMsg.body.length > 24 ? "…" : ""}`
              : (p.lastMessageAt ? `Last activity: ${new Date(toMs(p.lastMessageAt)).toLocaleString()}` : "No messages yet");

            return (
              <div
                key={id}
                className={`${s.recipient} ${isActive ? s.recipientActive : ""}`}
                onClick={() => {
                  setActivePeer(id);
                  // Sync URL so that refresh/bookmark works
                  navigate(`/chat/${id}`, { replace: true });
                }}
                role="button"
                tabIndex={0}
              >
                <div className={s.avatar}>
                  <Avatar userId={p.userId} alt={p.userName} size={32} />
                </div>
                <div className={s.meta}>
                  <div className={s.name}>
                    {p.userName}
                    {p.isOnline && <span className={s.onlineDot} aria-label="online" title="Online" />}
                    {!p.isOnline && <span className={s.offlineDot} aria-label="offline" title="Offline" />}
                    {/* Unread badge: dot or number; here we show a small numeric pill if >0 */}
                    {p.unreadMessages > 0 && (
                      <span className={s.unreadBadge} aria-label={`${p.unreadMessages} unread`}>
                        {p.unreadMessages > 99 ? "99+" : p.unreadMessages}
                      </span>
                    )}
                  </div>
                  <div className={s.sub}>{preview}</div>
                </div>
              </div>
            );
          })}
        </div>
      </aside>

      {/* --- MAIN PANEL --- */}
      <section className={`${s.panel} ${isMobile && !showMobileChatView ? s.hiddenOnMobile : ''} ${isMobile && showMobileChatView ? s.mobileModal : ''}`}>
        <header className={s.header}>
          {/* Mobile back button */}
          {isMobile && showMobileChatView && (
            <button 
              className={s.backButton}
              onClick={handleBackToList}
              aria-label="Back to chat list"
            >
              <ArrowLeft size={20} />
            </button>
          )}
          <div className={s.headerTitle}>
            <div 
              className={s.avatar}
              onClick={() => activePeerSummary && handleOpenProfile(activePeerSummary.userId)}
              style={{ cursor: activePeerSummary ? 'pointer' : 'default' }}
              role={activePeerSummary ? 'button' : undefined}
              tabIndex={activePeerSummary ? 0 : undefined}
              onKeyDown={(e) => {
                if (activePeerSummary && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  handleOpenProfile(activePeerSummary.userId);
                }
              }}
            >
              {activePeerSummary ? (
                <Avatar
                  userId={activePeerSummary.userId}
                  alt={activePeerSummary.userName}
                  size={36}
                />
              ) : (
                // Placeholder
                <div style={{ width: 36, height: 36, borderRadius: 8, background: "var(--surface-3)" }} />
              )}
              
            </div>
            <div>
              <div className={s.name}>
                {activePeer != null ? nameFor(activePeer) : "Select a conversation"}
              </div>
            </div>
          </div>
        </header>

        <div ref={messagesEndRef} className={s.messages}>
            {activeConvo?.hasMore && showLoadOlderButton && (
            <div className={s.loadMoreWrap}>
                <button
                  className="u-btn u-btn--primary"
                  onClick={loadOlder}
                  disabled={loadingOlder}
                  aria-busy={loadingOlder}
                  aria-live="polite"
                >
                  <History size={16} aria-hidden /> {loadingOlder ? "Loading…" : "Load older"}
                </button>
            </div>
            )}
          {(activeConvo?.messages || []).map((m) => {
            const mine = m.from === myId;
            return (
              <div key={m.id} className={`${s.messageRow} ${mine ? s.messageMine : ""}`}>
                <div className={s.messageBubble}>
                  <div>{m.body}</div>
                  <span className={s.time}>{formatTime(m.at)}</span>
                </div>
              </div>
            );
          })}

          {activeConvo?.peerTyping && <div className={s.typing}>Typing…</div>}
          {activePeer == null && (
            <div className={s.dayDivider}>Choose a connection on the left to start chatting.</div>
          )}
        </div>

        <div className={s.composer}>
          <textarea
            className={`u-input ${s.input}`}
            placeholder={activePeer == null ? "Select a conversation…" : "Type a message…"}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
              } else {
                sendTyping();
              }
            }}
            disabled={activePeer == null}
          />
          <button
            type="button"
            className="u-btn u-btn--primary"
            onClick={sendMessage}
            disabled={!canSend}
            // The button is available when we have a recipient and some text
            // If the socket is closed the message is added in the 'outbox' queue.
          >
            <Send size={18} aria-hidden />
          </button>
        </div>
      </section>

      {/* ProfileModal for viewing user profiles when clicking avatars */}
      <ProfileModal
        isOpen={selectedUserId !== null}
        userId={selectedUserId || 0}
        onClose={handleCloseProfile}
        {...(distance !== undefined && { distance })}
        actions={
          selectedUserId && (
            <button
              className="u-btn u-btn--primary"
              onClick={() => {
                // Close modal since we're already in chat with this user
                handleCloseProfile();
              }}
            >
              Close
            </button>
          )
        }
      />
    </div>
  );
};

export default Chat;
