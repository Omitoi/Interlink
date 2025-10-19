// Modern Recommendations page with UserCard grid and ProfileModal
import React, { useMemo, useState, useEffect } from "react";
import { Users, X, Heart } from "lucide-react";
import useRecommendations from "../hooks/useRecommendations";
import useMyProfile from "../hooks/useMyProfile";
import { getUserProfile } from "../api/users";
import { dismissCandidate, requestCandidate } from "../api/recommendations";
import { useToast } from "../hooks/useToast";
import { calculateDistanceIfAvailable } from "../utils/distance";
import type { ProfileOverview } from "../types/api";
import s from "./Recommendations.module.css";

import UserCard from "../components/UserCard";
import ProfileModal from "../components/ProfileModal";
import ToastContainer from "../components/ToastContainer";

const Recommendations: React.FC = () => {
  const { loading, error, candidates } = useRecommendations();
  const { profile: myProfile } = useMyProfile();
  const { toasts, confirm } = useToast();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [selectedUserProfile, setSelectedUserProfile] = useState<ProfileOverview | null>(null);
  
  // Optimistic local removals (ids hidden from the list immediately)
  const [hiddenIds, setHiddenIds] = useState<Set<number>>(new Set());

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

  const visible = useMemo(
    () => candidates.filter(c => !hiddenIds.has(c.id)),
    [candidates, hiddenIds]
  );

  const currentCandidate = visible[currentIndex];

  const handleOpenProfile = (userId: number, index: number) => {
    setSelectedUserId(userId);
    setCurrentIndex(index);
  };

  const handleCloseProfile = () => {
    setSelectedUserId(null);
  };

  const nextCandidate = () => {
    const nextIndex = currentIndex + 1;
    if (nextIndex < visible.length) {
      const next = visible[nextIndex];
      if (next) {
        handleOpenProfile(next.id, nextIndex);
      }
    } else {
      handleCloseProfile();
    }
  };

  async function onDismiss(id: number) {
    const shouldDismiss = await confirm(
      "Dismiss Recommendation",
      "Are you sure you want to dismiss this recommendation? You won't see them again.",
      {
        type: "danger",
        confirmText: "Dismiss",
        cancelText: "Cancel"
      }
    );

    if (!shouldDismiss) return;

    // Optimistic: hide immediately
    setHiddenIds(prev => new Set(prev).add(id));
    
    try {
      await dismissCandidate(id);
      nextCandidate();
    } catch (e: unknown) {
      // Revert if server failed
      setHiddenIds(prev => {
        const copy = new Set(prev);
        copy.delete(id);
        return copy;
      });
      const error = e as { response?: { data?: { message?: string } }; message?: string };
      console.error("Failed to dismiss:", error?.response?.data?.message || error?.message || "Unknown error");
    }
  }

  async function onRequest(id: number) {
    const shouldRequest = await confirm(
      "Send Connection Request",
      "Would you like to send a connection request to this person?",
      {
        type: "info",
        confirmText: "Send Request",
        cancelText: "Cancel"
      }
    );

    if (!shouldRequest) return;

    // Optimistic: hide immediately
    setHiddenIds(prev => new Set(prev).add(id));
    
    try {
      const res = await requestCandidate(id);
      if (res.state === "accepted") {
        // Matched! Connection established
      } else if (res.state === "pending") {
        // Request sent (pending approval)
      } else {
        // mismatch: revert (the other side may have declined/dismissed)
        setHiddenIds(prev => {
          const copy = new Set(prev);
          copy.delete(id);
          return copy;
        });
      }
      nextCandidate();
    } catch (e: unknown) {
      // true error -> revert
      setHiddenIds(prev => {
        const copy = new Set(prev);
        copy.delete(id);
        return copy;
      });
      const error = e as { response?: { data?: { message?: string } }; message?: string };
      console.error("Failed to send request:", error?.response?.data?.message || error?.message || "Unknown error");
    }
  }

  if (loading) return <div className={s.loading}>Loading recommendations…</div>;
  if (error) return (
    <div className={s.error}>
      <div className={s.errorText}>{error}</div>
    </div>
  );

  return (
    <main className={s.page}>
      <header className={s.header}>
        <h1 className={s.title}>
          <Users size={24} /> Recommendations
        </h1>
      </header>

      {visible.length === 0 ? (
        <div className={s.empty}>
          <Users size={48} className={s.emptyIcon} />
          <h3>No recommendations available</h3>
          <p>Check back later for new connection opportunities!</p>
        </div>
      ) : (
        <div className={s.recommendations}>
          {visible.map((candidate, index) => (
            <UserCard
              key={candidate.id}
              user={{
                id: candidate.id,
                display_name: candidate.display_name,
                profile_picture: candidate.profile_picture ?? null,
                is_online: candidate.is_online
              }}
              {...(candidate.score_percentage !== undefined && { scorePercentage: candidate.score_percentage })}
              onClick={() => handleOpenProfile(candidate.id, index)}
            />
          ))}
        </div>
      )}

            {selectedUserId !== null && currentCandidate && (
        <div className={s.fixedActions}>
          <button
            onClick={() => onDismiss(currentCandidate.id)}
            className={`${s.actionButton} ${s.dismissButton}`}
            aria-label="Dismiss recommendation"
          >
            <X size={24} />
            <span>Dismiss</span>
          </button>
          <button
            onClick={() => onRequest(currentCandidate.id)}
            className={`${s.actionButton} ${s.requestButton}`}
            aria-label="Send connection request"
          >
            <Heart size={24} />
            <span>Connect</span>
          </button>
        </div>
      )}

      <ProfileModal
        isOpen={selectedUserId !== null}
        userId={selectedUserId || 0}
        onClose={handleCloseProfile}
        {...(distance !== undefined && { distance })}
        actions={
          selectedUserId && (
            <>
              <button
                className="u-btn"
                onClick={() => {
                  if (selectedUserId) {
                    onDismiss(selectedUserId);
                    handleCloseProfile();
                  }
                }}
              >
                Dismiss
              </button>
              <button
                className="u-btn u-btn--primary"
                onClick={() => {
                  if (selectedUserId) {
                    onRequest(selectedUserId);
                    handleCloseProfile();
                  }
                }}
              >
                Request Connection
              </button>
            </>
          )
        }
      />

      <ToastContainer toasts={toasts} />
    </main>
  );
};

export default Recommendations;
