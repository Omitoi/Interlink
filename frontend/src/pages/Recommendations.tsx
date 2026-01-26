import React, { useMemo, useState, useEffect } from "react";
import { Users } from "lucide-react";
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
  const { toasts, confirm, success } = useToast();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
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

  const visible = useMemo(
    () => candidates.filter(c => !hiddenIds.has(c.id)),
    [candidates, hiddenIds]
  );

  const handleOpenProfile = (userId: number) => {
    setSelectedUserId(userId);
    // index unused now
  };

  const handleCloseProfile = () => {
    setSelectedUserId(null);
  };


  async function onDismiss(id: number) {
    const shouldDismiss = await confirm(
      "Dismiss Recommendation",
      "Are you sure you want to dismiss this recommendation?",
      { type: "danger", confirmText: "Dismiss", cancelText: "Cancel" }
    );

    if (!shouldDismiss) return;

    setHiddenIds(prev => new Set(prev).add(id));

    try {
      await dismissCandidate(id);
      // Ensure we don't get stuck if we just dismissed the current one displayed in main view?
      // "Recommendations" page displays a grid, so "nextCandidate" logic is for the modal flow mainly?
      // Actually standard view is grid. Next logic is okay.
    } catch (e: unknown) {
      setHiddenIds(prev => {
        const copy = new Set(prev);
        copy.delete(id);
        return copy;
      });
      console.error("Failed to dismiss");
    }
  }

  // Updated: Replaced particles with Toast message
  async function onRequest(id: number) {
    // Optimistic hide
    setHiddenIds(prev => new Set(prev).add(id));

    try {
      await requestCandidate(id);
      success("Connection Request Sent", "We've notified them!");
    } catch (e: unknown) {
      // Revert on error
      setHiddenIds(prev => {
        const copy = new Set(prev);
        copy.delete(id);
        return copy;
      });
      console.error("Failed to send request");
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
          {visible.map((candidate) => (
            <UserCard
              key={candidate.id}
              user={{
                id: candidate.id,
                display_name: candidate.display_name,
                profile_picture: candidate.profile_picture ?? null,
                is_online: candidate.is_online
              }}
              {...(candidate.score_percentage !== undefined && { scorePercentage: candidate.score_percentage })}
              onClick={() => handleOpenProfile(candidate.id)}
            />
          ))}
        </div>
      )}

      {/* Floating Action Bar logic was tied to "currentCandidate" which implies a slideshow view? 
          But the main view renders a grid. 
          The code block had "selectedUserId !== null && currentCandidate" logic. 
          Ideally these actions appear in the Modal? 
          Or maybe there's a floating bar component?
          Reviewing original code: it showed a fixed bar if someone is selected.
          Let's align it.
       */}

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
                    // Pass event for animation coords
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
