// src/pages/Dashboard.tsx
import React, { useState, useMemo, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import useConnections from "../hooks/useConnections";
import useMyProfile from "../hooks/useMyProfile";
import { useToast } from "../hooks/useToast";
import { getUserProfile } from "../api/users";
import { calculateDistanceIfAvailable } from "../utils/distance";
import type { ConnectionDisplay } from "../types/domain";
import type { UserPublic, ProfileOverview } from "../types/api";

import ConnectionListSection from "../components/ConnectionListSection";
import UserCard from "../components/UserCard";
import ProfileModal from "../components/ProfileModal";
import ToastContainer from "../components/ToastContainer";
import p from "./Dashboard.module.css";

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const { toasts, confirm } = useToast();
  const { profile: myProfile } = useMyProfile();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [selectedUserPublic, setSelectedUserPublic] = useState<(Partial<UserPublic> & { id: number }) | null>(null);
  const [selectedUserProfile, setSelectedUserProfile] = useState<ProfileOverview | null>(null);
  
  const {
    loading,
    error,
    connections,
    requests,
    busy,
    accept,
    decline,
    remove,
  } = useConnections();

  const handleRemoveConnection = async (connectionId: number, userName: string) => {
    const shouldRemove = await confirm(
      "Remove Connection",
      `Are you sure you want to remove your connection with ${userName}? This action cannot be undone.`,
      {
        type: "danger",
        confirmText: "Remove",
        cancelText: "Cancel"
      }
    );

    if (shouldRemove) {
      remove(connectionId);
    }
  };

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

  const handleDeclineRequest = async (requestId: number, userName: string) => {
    const shouldDecline = await confirm(
      "Decline Connection Request",
      `Are you sure you want to decline the connection request from ${userName}?`,
      {
        type: "warning",
        confirmText: "Decline",
        cancelText: "Cancel"
      }
    );

    if (shouldDecline) {
      decline(requestId);
    }
  };

  const handleOpenProfile = (user: { id: number; display_name: string; profile_picture?: string | null }) => {
    setSelectedUserPublic({
      id: user.id,
      display_name: user.display_name,
      profile_picture: user.profile_picture,
    });
    setSelectedUserId(user.id);
  };

  const handleCloseProfile = () => {
    setSelectedUserId(null);
    setSelectedUserPublic(null);
  };

  const ErrorBanner = error ? (
    <div role="alert" style={{ marginBottom: 12 }}>
      <strong>Virhe:</strong> {error}
    </div>
  ) : null;

  return (
    <main className={p.page}>
      <div className={p.pageInner}>
        <header style={{ marginBottom: 16 }}>
          <h1>Dashboard</h1>
        </header>

        {ErrorBanner}

        <ConnectionListSection
          title="Connection requests"
          loading={loading}
          emptyText="No incoming requests"
        >
          {requests.map((req) => (
            <UserCard
              key={req.user.id}
              user={req.user}
              onClick={() => handleOpenProfile(req.user)}
              actions={
                <>
                  <button
                    className="u-btn u-btn--primary"
                    onClick={() => accept(req.id)}
                    disabled={!!busy[req.user.id]}
                    title="Accept"
                  >
                    Accept
                  </button>
                  <button
                    className="u-btn"
                    onClick={() => handleDeclineRequest(req.id, req.user.display_name)}
                    disabled={!!busy[req.user.id]}
                    title="Decline"
                  >
                    Decline
                  </button>
                </>
              }
            />
          ))}
        </ConnectionListSection>

        <ConnectionListSection
          title="Connections"
          loading={loading}
          emptyText="No connections yet"
        >
          {connections.map((conn: ConnectionDisplay) => (
            <UserCard
              key={conn.id}
              user={conn.user}
              onClick={() => handleOpenProfile(conn.user)}
              actions={
                <>
                  <button
                    className="u-btn"
                    onClick={() => navigate(`/chat/${conn.user.id}`)}
                    title="Open chat"
                  >
                    Chat
                  </button>
                  <button
                    className="u-btn u-btn--danger"
                    onClick={() => handleRemoveConnection(conn.user.id, conn.user.display_name)}
                    disabled={!!busy[conn.id]}
                    title="Remove connection"
                  >
                    Remove
                  </button>
                </>
              }
            />
          ))}
        </ConnectionListSection>
      </div>
      
      <ProfileModal
        userId={selectedUserId!}
        isOpen={selectedUserId !== null}
        onClose={handleCloseProfile}
        initialPublic={selectedUserPublic ?? undefined}
        {...(distance !== undefined && { distance })}
        actions={
          selectedUserId && requests.find(req => req.user.id === selectedUserId) ? (
            <>
              <button
                className="u-btn u-btn--primary"
                onClick={() => {
                  const req = requests.find(r => r.user.id === selectedUserId);
                  if (req) {
                    accept(req.id);
                    handleCloseProfile();
                  }
                }}
                disabled={!!busy[selectedUserId]}
              >
                Accept Connection
              </button>
              <button
                className="u-btn"
                onClick={() => {
                  const req = requests.find(r => r.user.id === selectedUserId);
                  if (req) {
                    decline(req.id);
                    handleCloseProfile();
                  }
                }}
                disabled={!!busy[selectedUserId]}
              >
                Decline
              </button>
            </>
          ) : selectedUserId && connections.find(conn => conn.user.id === selectedUserId) ? (
            <>
              <button
                className="u-btn u-btn--primary"
                onClick={() => {
                  navigate(`/chat/${selectedUserId}`);
                  handleCloseProfile();
                }}
              >
                Open Chat
              </button>
              <button
                className="u-btn u-btn--danger"
                onClick={() => {
                  const conn = connections.find(c => c.user.id === selectedUserId);
                  if (conn) {
                    handleRemoveConnection(conn.user.id, conn.user.display_name);
                    handleCloseProfile();
                  }
                }}
                disabled={selectedUserId ? !!busy[selectedUserId] : false}
              >
                Remove Connection
              </button>
            </>
          ) : null
        }
      />
      
      <ToastContainer toasts={toasts} />
    </main>
  );
};

export default Dashboard;
