import React, { useState } from 'react';
import { Send } from 'lucide-react';
import s from '../pages/Chat.module.css'; // Reusing existing styles

interface ChatComposerProps {
    onSend: (message: string) => void;
    onTyping: () => void;
    disabled: boolean;
}

const ChatComposer: React.FC<ChatComposerProps> = ({ onSend, onTyping, disabled }) => {
    const [draft, setDraft] = useState("");

    const handleSend = () => {
        const body = draft.trim();
        if (!body) return;
        onSend(body);
        setDraft("");
    };

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        } else {
            onTyping();
        }
    };

    return (
        <div className={s.composer}>
            <textarea
                className={`u-input ${s.input}`}
                placeholder={disabled ? "Select a conversation…" : "Type a message…"}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={handleKeyDown}
                disabled={disabled}
            />
            <button
                type="button"
                className="u-btn u-btn--primary"
                onClick={handleSend}
                disabled={disabled || draft.trim().length === 0}
            >
                <Send size={18} aria-hidden />
            </button>
        </div>
    );
};

export default React.memo(ChatComposer);
