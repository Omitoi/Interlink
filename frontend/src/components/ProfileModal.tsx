// src/components/ProfileModal.tsx
import React, { useEffect, useId, useState } from "react";
import { X } from "lucide-react";
import type { UserPublic, ProfileOverview, UserBiography } from "../types/api";
import { getUserPublic, getUserProfile } from "../api/users";
import api from "../api/axios";
import Avatar from "./Avatar";
import s from "./ProfileModal.module.css";

export type ProfileModalProps = {
  userId: number;
  isOpen: boolean;
  onClose: () => void;
  /**
   * Optional shell data to avoid an extra round‑trip.
   * If omitted, the component will fetch /users/{id} for the header.
   */
  initialPublic?: (Partial<UserPublic> & { id: number }) | undefined;
  /** Optional actions to display at the bottom */
  actions?: React.ReactNode;
  /** Optional distance in meters to display in the header */
  distance?: number;
};

// --- API helper for /users/{id}/bio ----------------------------------------
async function getUserBio(id: number): Promise<UserBiography> {
  const { data } = await api.get<UserBiography>(`/users/${id}/bio`);
  return data;
}

// --- Small utils ------------------------------------------------------------
function formatDistance(distanceInMeters: number): string {
  if (distanceInMeters < 1000) {
    return `${Math.round(distanceInMeters)}m`;
  } else {
    const km = distanceInMeters / 1000;
    if (km < 10) {
      return `${km.toFixed(1)}km`;
    } else {
      return `${Math.round(km)}km`;
    }
  }
}

function OnlineDot({ online }: { online: boolean }) {
  return <span className={online ? s.dotOnline : s.dotOffline} aria-label={online ? "online" : "offline"} />;
}

function BioRow({ label, items }: { label: string; items: string[] | string }) {
  // Some Biography items are lists, some strings. Handle differently
  if (typeof items === "string") {
    return (
      <div className={s.bioSection}>
        <h4>{label}</h4>
        <div>{items}</div>
      </div>
    );
  }

  if (!items?.length) return null;

  return (
    <div className={s.bioSection}>
      <h4>{label}</h4>
      <ul>
        {items.map((t, i) => (
          <li key={i}>{t}</li>
        ))}
      </ul>
    </div>
  );
}

// --- Component --------------------------------------------------------------
const ProfileModal: React.FC<ProfileModalProps> = ({
  userId,
  isOpen,
  onClose,
  initialPublic,
  actions,
  distance,
}) => {
  const [loadingHeader, setLoadingHeader] = useState(!initialPublic);
  const [loadingProfile, setLoadingProfile] = useState(false);
  const [loadingBio, setLoadingBio] = useState(false);

  const [publicData, setPublicData] = useState<UserPublic | null>(
    initialPublic
      ? ({
          id: userId,
          display_name: initialPublic.display_name ?? "",
          profile_picture: initialPublic.profile_picture ?? null,
        } as UserPublic)
      : null
  );
  const [profile, setProfile] = useState<ProfileOverview | null>(null);
  const [bio, setBio] = useState<UserBiography | null>(null);
  const [error, setError] = useState<string | null>(null);

  const modalId = useId();

  // Fetch header shell if not provided
  useEffect(() => {
    if (!isOpen || publicData) return;

    const fetchHeader = async () => {
      try {
        setLoadingHeader(true);
        setError(null);
        const userData = await getUserPublic(userId);
        setPublicData(userData);
      } catch (err) {
        console.error("Failed to load user data:", err);
        setError("Failed to load user information");
      } finally {
        setLoadingHeader(false);
      }
    };

    fetchHeader();
  }, [userId, publicData, isOpen]);

  // Auto-load profile and bio when modal opens
  useEffect(() => {
    if (!isOpen || profile || loadingProfile) return;

    const fetchProfile = async () => {
      try {
        setLoadingProfile(true);
        setError(null);
        const profileData = await getUserProfile(userId);
        setProfile(profileData);
        
        // Also load bio immediately after profile
        if (!bio && !loadingBio) {
          setLoadingBio(true);
          try {
            const bioData = await getUserBio(userId);
            setBio(bioData);
          } catch (bioErr) {
            console.error("Failed to load bio:", bioErr);
            // Don't set error for bio failure, it's not critical
          } finally {
            setLoadingBio(false);
          }
        }
      } catch (err) {
        console.error("Failed to load profile:", err);
        setError("Failed to load profile information");
      } finally {
        setLoadingProfile(false);
      }
    };

    fetchProfile();
  }, [userId, profile, loadingProfile, isOpen, bio, loadingBio]);

// Handle escape key and backdrop click
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
    }

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "unset";
    };
  }, [isOpen, onClose]);

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setProfile(null);
      setBio(null);
      setError(null);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className={s.overlay} onClick={onClose}>
      {/* Fixed close button - overlays entire modal */}
      <button
        className={s.closeButton}
        onClick={onClose}
        aria-label="Close profile"
        title="Close"
      >
        <X size={48} />
      </button>
      
      <div 
        className={s.modal} 
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${modalId}-title`}
      >
        {/* Modal content */}
        <div className={s.content}>
          {loadingHeader && (
            <div className={s.loading}>Loading user information...</div>
          )}

          {!loadingHeader && publicData && (
            <>
              {/* Header with full-width profile picture */}
              <header className={s.header}>
                <div className={s.profilePictureContainer}>
                  <Avatar
                    userId={userId}
                    alt={publicData.display_name}
                    size={400}
                  />
                  
                  {/* Online status overlay - top left */}
                  {profile && profile.is_online !== undefined && (
                    <div className={s.statusOverlay}>
                      <OnlineDot online={profile.is_online} />
                      <span>{profile.is_online ? "Online" : "Offline"}</span>
                    </div>
                  )}
                  
                  {/* Name overlay - bottom */}
                  <div className={s.nameOverlay}>
                    <div className={s.nameRow}>
                      <h1 id={`${modalId}-title`} className={s.name}>
                        {publicData.display_name}
                      </h1>
                      {distance !== undefined && (
                        <div className={s.distance}>
                          {formatDistance(distance)}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </header>

              {/* Profile content */}
              {loadingProfile && (
                <div className={s.loading}>Loading profile...</div>
              )}

              {!loadingProfile && profile && (
                <div className={s.profileBody}>
                  {profile.about_me && (
                    <section className={s.aboutSection}>
                      <h2>About</h2>
                      <p className={s.about}>{profile.about_me}</p>
                    </section>
                  )}

                  {(bio || loadingBio) && (
                    <section className={s.bio} aria-label="Full biography">
                      <h2>Biography</h2>
                      {loadingBio ? (
                        <div className={s.loading}>Loading biography...</div>
                      ) : bio ? (
                        <div className={s.bioContent}>
                          <BioRow label="Analog passions" items={bio.analog_passions} />
                          <BioRow label="Digital delights" items={bio.digital_delights} />
                          <BioRow label="Seeking collaboration" items={bio.collaboration_interests} />
                          <BioRow label="Favorite food" items={bio.favorite_food} />
                          <BioRow label="Favorite music" items={bio.favorite_music} />
                        </div>
                      ) : null}
                    </section>
                  )}
                </div>
              )}

              {/* Actions */}
              {actions && (
                <footer className={s.actions}>
                  {actions}
                </footer>
              )}
            </>
          )}

          {error && (
            <div role="alert" className={s.error}>
              {error}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default ProfileModal;