import React, { useEffect, useMemo, useRef, useState, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import useMyProfile from "../hooks/useMyProfile";
import { getUserProfile } from "../api/users";
import { calculateDistanceIfAvailable } from "../utils/distance";
import { useChatStore } from "../stores/useChatStore";
import s from "./Chat.module.css";
import { ArrowLeft } from 'lucide-react'
import Avatar from "../components/Avatar"
import ProfileModal from "../components/ProfileModal"
import ChatComposer from "../components/ChatComposer"
import MessageList from "../components/MessageList"
import { ProfileOverview } from "../types/api";

const Chat: React.FC = () => {
  const { token, user } = useAuth();
  const { profile: myProfile } = useMyProfile();
  const myId = user?.id;

  const { peerId } = useParams<{ peerId?: string }>();
  const navigate = useNavigate();

  // Access Store
  const {
    peers,
    activePeer,
    convos,
    loadingOlder,
    initSocket,
    closeSocket,
    loadPeers,
    setActivePeer,
    loadOlderMessages,
    sendMessage,
    sendTyping
  } = useChatStore();

  // ProfileModal state
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [selectedUserProfile, setSelectedUserProfile] = useState<ProfileOverview | null>(null);

  // Mobile state
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 1024);
  const [showMobileChatView, setShowMobileChatView] = useState(false);

  // Initialize Socket on mount (or when token/myId avail)
  useEffect(() => {
    if (token && myId) {
      initSocket(token, myId);
      loadPeers();
    }
    return () => {
      // We might not want to close socket on every unmount if we want persistence across nav?
      // But for now, let's close it to be safe/clean.
      closeSocket();
    };
  }, [token, myId]);

  // Handle URL param sync with Store
  useEffect(() => {
    if (!peerId) {
      if (isMobile) {
        setTimeout(() => setShowMobileChatView(false), 0);
      } else {
        // Default to first peer if desktop and no peer selected?
        // The store loading logic usually handles "lastPeerId" from localstorage or first.
        // But if we want to enforce URL -> Store:
        setActivePeer(null);
      }
      return;
    }

    const idNum = Number(peerId);
    if (!Number.isNaN(idNum)) {
      setActivePeer(idNum);
      if (isMobile) setShowMobileChatView(true);
    }
  }, [peerId, isMobile]);

  // Handle Mobile Resize
  useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth <= 1024;
      setIsMobile(mobile);
      if (!mobile && showMobileChatView) setShowMobileChatView(false);
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [showMobileChatView]);


  // Profile Logic
  useEffect(() => {
    if (selectedUserId) {
      getUserProfile(selectedUserId)
        .then(setSelectedUserProfile)
        .catch(() => setSelectedUserProfile(null));
    } else {
      const timer = setTimeout(() => setSelectedUserProfile(null), 0);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [selectedUserId]);

  const distance = useMemo(() => {
    if (!myProfile || !selectedUserProfile) return undefined;
    return calculateDistanceIfAvailable(
      myProfile.location_lat,
      myProfile.location_lon,
      selectedUserProfile.location_lat,
      selectedUserProfile.location_lon
    );
  }, [myProfile, selectedUserProfile]);

  const handleOpenProfile = useCallback((userId: number) => setSelectedUserId(userId), []);
  const handleCloseProfile = useCallback(() => setSelectedUserId(null), []);

  // Back to list (Mobile)
  const handleBackToList = useCallback(() => {
    setActivePeer(null); // Clear store selection
    setShowMobileChatView(false);
    navigate('/chat', { replace: true });
  }, [navigate, setActivePeer]);

  // Derived Values
  const activeConvo = activePeer ? convos[activePeer] : undefined;

  // Helpers
  const nameFor = (peerUserId: number) =>
    peers.find(p => p.userId === peerUserId)?.userName ?? `User ${peerUserId}`;

  const activePeerSummary = activePeer ? peers.find(p => p.userId === activePeer) : undefined;

  // Typing debounce (local ref)
  const typingDebounceRef = useRef<number | null>(null);
  const onTyping = useCallback(() => {
    if (typingDebounceRef.current) return;
    sendTyping();
    typingDebounceRef.current = window.setTimeout(() => {
      typingDebounceRef.current = null;
    }, 500);
  }, [sendTyping]);

  // Scroll detection for "Load Older"
  const [showLoadOlderButton, setShowLoadOlderButton] = useState(false);
  const handleNearTop = useCallback((isNear: boolean) => {
    setShowLoadOlderButton(isNear);
  }, []);


  return (
    <div className={`${s.layout} ${isMobile ? s.mobileLayout : ''}`}>
      {/* Sidebar */}
      <aside className={`${s.sidebar} ${isMobile && showMobileChatView ? s.hiddenOnMobile : ''}`}>
        <div className={s.sidebarHeader}>Connections</div>
        {peers.length === 0 && <div className={s.sub}>No connections yet.</div>}

        <div className={s.recipientList}>
          {peers.map((p) => {
            const id = p.userId;
            const isActive = activePeer === id;

            // Preview from store
            const c = convos[id];
            const lastMsg = c?.messages?.[c.messages.length - 1];
            const preview = lastMsg
              ? `Last: ${lastMsg.body.slice(0, 24)}${lastMsg.body.length > 24 ? "…" : ""}`
              : (p.lastMessageAt ? `Last activity: ${new Date(p.lastMessageAt).toLocaleTimeString()}` : "No messages yet");

            return (
              <div
                key={id}
                className={`${s.recipient} ${isActive ? s.recipientActive : ""}`}
                onClick={() => {
                  setActivePeer(id);
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
                    {p.isOnline ? <span className={s.onlineDot} title="Online" /> : <span className={s.offlineDot} title="Offline" />}
                    {p.unreadMessages > 0 && (
                      <span className={s.unreadBadge}>{p.unreadMessages > 99 ? "99+" : p.unreadMessages}</span>
                    )}
                  </div>
                  <div className={s.sub}>{preview}</div>
                </div>
              </div>
            );
          })}
        </div>
      </aside>

      {/* Main Panel */}
      <section className={`${s.panel} ${isMobile && !showMobileChatView ? s.hiddenOnMobile : ''} ${isMobile && showMobileChatView ? s.mobileModal : ''}`}>
        <header className={s.header}>
          {isMobile && showMobileChatView && (
            <button className={s.backButton} onClick={handleBackToList}>
              <ArrowLeft size={20} />
            </button>
          )}
          <div className={s.headerTitle}>
            <div className={s.avatar} onClick={() => activePeerSummary && handleOpenProfile(activePeerSummary.userId)} style={{ cursor: activePeerSummary ? 'pointer' : 'default' }}>
              {activePeerSummary ? <Avatar userId={activePeerSummary.userId} alt={activePeerSummary.userName} size={36} /> : <div style={{ width: 36, height: 36 }} />}
            </div>
            <div className={s.name}>
              {activePeer ? nameFor(activePeer) : "Select a conversation"}
            </div>
          </div>
        </header>

        <MessageList
          messages={activeConvo?.messages || []}
          myId={myId}
          peerTyping={activeConvo?.peerTyping || false}
          activePeer={activePeer}
          hasMore={activeConvo?.hasMore}
          loadingOlder={loadingOlder}
          onLoadOlder={loadOlderMessages}
          showLoadOlderButton={showLoadOlderButton}
          onNearTop={handleNearTop}
        />

        <ChatComposer
          onSend={sendMessage}
          onTyping={onTyping}
          disabled={activePeer == null}
        />
      </section>

      {/* Profile Modal */}
      <ProfileModal
        isOpen={selectedUserId !== null}
        userId={selectedUserId || 0}
        onClose={handleCloseProfile}
        {...(distance !== undefined && { distance })}
        actions={selectedUserId && (
          <button className="u-btn u-btn--primary" onClick={handleCloseProfile}>Close</button>
        )}
      />
    </div>
  );
};

export default Chat;
