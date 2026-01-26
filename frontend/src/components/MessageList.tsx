import React, { useRef, useEffect } from 'react';
import { History } from 'lucide-react';
import s from '../pages/Chat.module.css';
import type { Message } from '../stores/useChatStore';

// Duplicate of formatTime to avoid heavy refactor of types yet
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

interface MessageListProps {
    messages: Message[];
    myId: number | undefined;
    peerTyping: boolean;
    activePeer: number | null;
    hasMore: boolean | undefined;
    loadingOlder: boolean;
    onLoadOlder: () => void;
    showLoadOlderButton: boolean;
    onNearTop: (isNear: boolean) => void;
}

const MessageList: React.FC<MessageListProps> = ({
    messages,
    myId,
    peerTyping,
    activePeer,
    hasMore,
    loadingOlder,
    onLoadOlder,
    showLoadOlderButton,
    onNearTop,
}) => {
    const messagesEndRef = useRef<HTMLDivElement>(null);

    // Auto-scroll bottom on new messages (simple implementation)
    useEffect(() => {
        if (loadingOlder) return;
        messagesEndRef.current?.scrollTo({ top: messagesEndRef.current.scrollHeight, behavior: "smooth" });
    }, [messages, loadingOlder]); // Scroll when messages change

    return (
        <div
            ref={messagesEndRef}
            className={s.messages}
            onScroll={(e) => {
                const target = e.currentTarget;
                const isNear = target.scrollTop <= 100;
                onNearTop(isNear);
            }}
        >
            {hasMore && showLoadOlderButton && (
                <div className={s.loadMoreWrap}>
                    <button
                        className="u-btn u-btn--primary"
                        onClick={onLoadOlder}
                        disabled={loadingOlder}
                        aria-busy={loadingOlder}
                        aria-live="polite"
                    >
                        <History size={16} aria-hidden /> {loadingOlder ? "Loading…" : "Load older"}
                    </button>
                </div>
            )}

            {messages.map((m) => {
                const mine = m.from === myId;
                return (
                    <div key={m.id} className={`${s.messageRow} ${mine ? s.messageMine : ""}`}>
                        <div
                            className={s.messageBubble}
                            style={{ opacity: m.status === 'sending' ? 0.5 : 1 }}
                        >
                            <div>{m.body}</div>
                            <span className={s.time}>{formatTime(m.at)}</span>
                            {m.status === 'error' && <span style={{ color: 'red', fontSize: '0.8em', marginLeft: '5px' }}>!</span>}
                        </div>
                    </div>
                );
            })}

            {peerTyping && <div className={s.typing}>Typing…</div>}
            {activePeer == null && (
                <div className={s.dayDivider}>Choose a connection on the left to start chatting.</div>
            )}
        </div>
    );
};

// Use deep compare or standard Reference equality?
// React.memo uses shallow compare by default.
// `messages` array ref changes on every new message -> re-render.
// `messages` array ref does NOT change on typing in parent input -> NO re-render.
// This is exactly what we want.
export default React.memo(MessageList);
