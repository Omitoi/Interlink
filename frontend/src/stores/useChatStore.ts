import { create } from 'zustand';
import { openChatSocket } from '../api/ws';
import { fetchChatHistory } from '../api/chat';
import api from '../api/axios';
import type { ChatMessage, ServerEvent } from '../types/chat';

const PAGE_SIZE = 50;

/** UI Message shape */
export type Message = {
    id: string; // uuid
    from: number;
    to: number;
    body: string;
    at: number;
    status?: 'sending' | 'sent' | 'error'; // Optimistic status
};

export type Conversation = {
    peerId: number;
    messages: Message[];
    peerTyping: boolean;
    hasMore?: boolean;
    oldestAt?: number;
};

export type ChatPeerSummary = {
    userId: number;
    userName: string;
    profilePicture?: string | null;
    lastMessageAt?: string | null;
    unreadMessages: number;
    isOnline: boolean;
};

type ChatStore = {
    // State
    socket: WebSocket | null;
    activePeer: number | null;
    peers: ChatPeerSummary[]; // Sidebar list
    convos: Record<number, Conversation>;
    myId: number | null;
    loadingOlder: boolean;

    // Actions
    initSocket: (token: string, myId: number) => void;
    closeSocket: () => void;
    loadPeers: () => Promise<void>;
    setActivePeer: (peerId: number | null) => void;
    loadChatHistory: (peerId: number) => Promise<void>;
    loadOlderMessages: () => Promise<void>;
    sendMessage: (text: string) => void;
    sendTyping: () => void;
    markRead: (peerId: number) => void;
};

// Helpers
function toMs(ts?: string | null): number {
    return ts ? Date.parse(ts) : 0;
}

export const useChatStore = create<ChatStore>((set, get) => ({
    socket: null,
    activePeer: null,
    peers: [],
    convos: {},
    myId: null,
    loadingOlder: false,

    initSocket: (token, myId) => {
        if (get().socket) return;

        // Optimistic queue handled internally by just checking readyState on send
        const socket = openChatSocket(token);

        // Handlers
        socket.onmessage = (evt) => {
            try {
                const ev: ServerEvent = JSON.parse(evt.data);
                if (ev.type === "message") {
                    const m = ev.data as ChatMessage;
                    const { activePeer, markRead } = get();

                    // Determine peer ID
                    const peer = m.from === myId ? (m.to ?? activePeer ?? -1) : m.from;

                    // Message sent handling (optimistic)
                    const newMsg: Message = {
                        id: crypto.randomUUID(),
                        from: m.from,
                        to: peer,
                        body: m.body,
                        at: Date.parse(m.ts),
                        status: 'sent'
                    };

                    set(state => {
                        const c = state.convos[peer] ?? { peerId: peer, messages: [], peerTyping: false };
                        let msgs = c.messages;
                        if (m.from === myId) {
                            msgs = msgs.filter(existing => !(existing.status === 'sending' && existing.body === m.body));
                        }

                        return {
                            convos: {
                                ...state.convos,
                                [peer]: { ...c, messages: [...msgs, newMsg], peerTyping: false }
                            }
                        };
                    });

                    // Update sidebar
                    set(state => ({
                        peers: state.peers.map(p => {
                            if (p.userId !== peer) return p;
                            const incoming = m.from !== myId;
                            const isActive = state.activePeer === peer;
                            const nextUnread = incoming && !isActive ? (p.unreadMessages || 0) + 1 : p.unreadMessages || 0;
                            return {
                                ...p,
                                lastMessageAt: new Date(newMsg.at).toISOString(),
                                unreadMessages: nextUnread
                            };
                        })
                    }));

                    // Read receipt if active
                    if (m.from !== myId && activePeer === peer) {
                        markRead(peer);
                    }

                } else if (ev.type === "typing") {
                    const from = ev.from;
                    if (from != null) {
                        set(state => {
                            const c = state.convos[from] ?? { peerId: from, messages: [], peerTyping: false };
                            return { convos: { ...state.convos, [from]: { ...c, peerTyping: true } } };
                        });
                        // Clear after delay
                        setTimeout(() => {
                            set(state => {
                                const c = state.convos[from];
                                if (!c) return {};
                                return { convos: { ...state.convos, [from]: { ...c, peerTyping: false } } };
                            });
                        }, 1200);
                    }
                }

            } catch (e) {
                console.warn("WS error", e);
            }
        };

        set({ socket, myId });
    },

    closeSocket: () => {
        const { socket } = get();
        if (socket) socket.close();
        set({ socket: null });
    },

    loadPeers: async () => {
        try {
            const res = await api.get<ChatPeerSummary[]>("/chats/summary");
            const list = Array.isArray(res.data) ? res.data : [];
            // Sort
            const sorted = list.sort((a, b) => toMs(b.lastMessageAt) - toMs(a.lastMessageAt));
            set({ peers: sorted });
        } catch {
            set({ peers: [] });
        }
    },

    setActivePeer: (peerId) => {
        set({ activePeer: peerId });
        if (peerId !== null) {
            get().loadChatHistory(peerId);
            // Clear unread in sidebar locally
            set(state => ({
                peers: state.peers.map(p => p.userId === peerId ? { ...p, unreadMessages: 0 } : p)
            }));

            // Persist
            try { localStorage.setItem("lastPeerId", String(peerId)); } catch { }
        }
    },

    loadChatHistory: async (peerId) => {
        try {
            // For simplicity, just fetch newest page.
            // If we already have it, maybe skip? But we want fresh status.
            // Let's just overwrite for now to be safe, or merge. Overwrite is cleaner for MVP sync.
            const desc = await fetchChatHistory(peerId, { limit: PAGE_SIZE });
            const asc: Message[] = [...desc].reverse().map((m: ChatMessage) => ({
                id: crypto.randomUUID(),
                from: m.from,
                to: m.to ?? peerId,
                body: m.body,
                at: Date.parse(m.ts),
                status: 'sent'
            }));

            set(state => {
                const base: Conversation = {
                    peerId,
                    messages: asc,
                    peerTyping: false,
                    hasMore: desc.length === PAGE_SIZE,
                    ...(asc[0]?.at !== undefined ? { oldestAt: asc[0].at } : {})
                };
                return { convos: { ...state.convos, [peerId]: base } };
            });

        } catch { }
    },

    loadOlderMessages: async () => {
        const { activePeer, convos, loadingOlder } = get();
        if (activePeer === null || loadingOlder) return;
        const convo = convos[activePeer];
        if (!convo || !convo.hasMore || !convo.oldestAt) return;

        set({ loadingOlder: true });
        try {
            const beforeISO = new Date(convo.oldestAt).toISOString();
            const desc = await fetchChatHistory(activePeer, { limit: PAGE_SIZE, before: beforeISO });
            const ascOlder: Message[] = [...desc].reverse().map((m: ChatMessage) => ({
                id: crypto.randomUUID(),
                from: m.from,
                to: m.to ?? activePeer,
                body: m.body,
                at: Date.parse(m.ts),
                status: 'sent'
            }));

            set(state => {
                const cur = state.convos[activePeer];
                if (!cur) return {};
                const newMsgs = [...ascOlder, ...cur.messages];


                const nextSafe: Conversation = {
                    peerId: cur.peerId,
                    messages: newMsgs,
                    peerTyping: cur.peerTyping,
                    hasMore: desc.length === PAGE_SIZE
                };
                const newOldest = ascOlder[0]?.at ?? cur.oldestAt;
                if (newOldest !== undefined) {
                    nextSafe.oldestAt = newOldest;
                }

                return { convos: { ...state.convos, [activePeer]: nextSafe } };
            });
        } finally {
            set({ loadingOlder: false });
        }
    },

    sendMessage: (text) => {
        const { socket, activePeer, myId } = get();
        if (!socket || activePeer === null || myId === null) return;
        const body = text.trim();
        if (!body) return;

        // 1. Optimistic Update
        const optimisticMsg: Message = {
            id: crypto.randomUUID(),
            from: myId,
            to: activePeer,
            body: body,
            at: Date.now(),
            status: 'sending'
        };

        set(state => {
            const c = state.convos[activePeer] ?? { peerId: activePeer, messages: [], peerTyping: false };
            return {
                convos: {
                    ...state.convos,
                    [activePeer]: { ...c, messages: [...c.messages, optimisticMsg] }
                }
            };
        });

        // 2. Send over socket
        try {
            socket.send(JSON.stringify({ type: "message", to: activePeer, body }));
        } catch {
            // Mark as error
            set(state => {
                const c = state.convos[activePeer];
                if (!c) return {};
                const msgs = c.messages.map(m => m.id === optimisticMsg.id ? { ...m, status: 'error' } : m);
                return { convos: { ...state.convos, [activePeer]: { ...c, messages: msgs as Message[] } } }; // cast status
            });
        }
    },

    sendTyping: () => {
        // Debounce logic can stay in component or move here. Moving here is cleaner.
        // But we need a ref for the timer. Zustand stores variables fine.
        // Let's implement debounce in component for now, or use a simple throttle here.
        const { socket, activePeer } = get();
        if (socket && activePeer) {
            socket.send(JSON.stringify({ type: "typing", to: activePeer }));
        }
    },

    markRead: (peerId) => {
        // Debounce?
        api.post("/chats/read", null, { params: { peer_id: peerId } }).catch(() => { });
    }
}));
