// ============================================================================
// File: src/components/CollapsibleUserCard.tsx
// Description: Reusable, accessible, collapsible user card for both
// Dashboard and Recommendations pages. Base view shows
// avatar, full name, and online status. On expand, it lazily
// loads /users/{id}/profile. A secondary control loads
// full biography from /users/{id}/bio.
// Uses CSS modules + existing api layer (axios) in this repo.
// ============================================================================


import React, { useCallback, useEffect, useId, useState } from "react";
import type { UserPublic, ProfileOverview, UserBiography } from "../types/api";
import s from "./CollapsibleUserCard.module.css";
import { getUserPublic, getUserProfile } from "../api/users";
import api from "../api/axios";
import { ChevronDown } from "lucide-react";
import Avatar from "./Avatar"

export type CollapsibleUserCardProps = {
userId: number;
/**
* Optional shell data to avoid an extra round‑trip.
* If omitted, the component will fetch /users/{id} for the header row.
*/
initialPublic?: Partial<UserPublic> & { id: number };
/** Called when the header row is clicked (after toggling). */
onToggleOpen?: (open: boolean) => void;
/** Optional right‑side actions for the collapsed header row */
headerActions?: React.ReactNode;

/**
* If true, auto‑expand on mount — useful if you deep‑link to a user.
*/
defaultOpen?: boolean;
};


// --- API helper for /users/{id}/bio ----------------------------------------
async function getUserBio(id: number): Promise<UserBiography> {
const { data } = await api.get<UserBiography>(`/users/${id}/bio`);
return data;
}


// --- Small utils ------------------------------------------------------------
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
    )
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
const CollapsibleUserCard: React.FC<CollapsibleUserCardProps> = ({
  userId,
  initialPublic,
  onToggleOpen,
  headerActions,
  defaultOpen = false,
}) => {
  const [open, setOpen] = useState(defaultOpen);
  const [loadingHeader, setLoadingHeader] = useState(!initialPublic);
  const [loadingProfile, setLoadingProfile] = useState(defaultOpen);
  const [loadingBio, setLoadingBio] = useState(false);

  const [publicData, setPublicData] = useState<UserPublic | null>(
    initialPublic ? ({ id: userId, display_name: initialPublic.display_name ?? "", profile_picture: initialPublic.profile_picture ?? null } as UserPublic) : null
  );
  const [profile, setProfile] = useState<ProfileOverview | null>(null);
  const [bio, setBio] = useState<UserBiography | null>(null);
  const [error, setError] = useState<string | null>(null);

  const panelId = useId();

  // Fetch header shell if not provided
  useEffect(() => {
    if (publicData) return;
    let ok = true;
    setLoadingHeader(true);
    getUserPublic(userId)
      .then((u) => ok && setPublicData(u))
      .catch((e) => ok && setError(e?.message || "Failed to load user"))
      .finally(() => ok && setLoadingHeader(false));
    return () => {
      ok = false;
    };
  }, [userId, publicData]);

  // Lazy‑fetch profile when expanding
  useEffect(() => {
    if (!open) return;
    if (profile) return;
    let ok = true;
    setLoadingProfile(true);
    getUserProfile(userId)
      .then((p) => ok && setProfile(p))
      .catch((e) => ok && setError(e?.message || "Failed to load profile"))
      .finally(() => ok && setLoadingProfile(false));
    return () => {
      ok = false;
    };
  }, [open, userId, profile]);

  const handleToggle = useCallback(() => {
    setOpen((prev) => {
      const next = !prev;
      onToggleOpen?.(next);
      return next;
    });
  }, [onToggleOpen]);

  const handleLoadBio = useCallback(async () => {
    if (bio || loadingBio) return;
    try {
      setLoadingBio(true);
      const b = await getUserBio(userId);
      setBio(b);
    } catch (e: unknown) {
      const error = e as { message?: string };
      setError(error?.message || "Failed to load biography");
    } finally {
      setLoadingBio(false);
    }
  }, [bio, loadingBio, userId]);

  const name = publicData?.display_name || profile?.display_name || "";
  const online = profile?.is_online ?? false; // Only trustworthy after /profile

  const onHeaderKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        handleToggle();
    }
  };

  return (
    <article className={`u-card ${s.card}`} data-open={open || undefined}>
        {/* == HEADER GRID == */}
        <div className={s.header}>

                <div
            role="button"
            tabIndex={0}
            aria-expanded={open}
            aria-controls={panelId}
            onClick={handleToggle}
            onKeyDown={onHeaderKeyDown}
            className={s.headerClickable}
        >
            <Avatar userId={userId} alt={name} size={48} />
            <div className={s.headerMeta}>
            <div className={s.nameRow}>
                <span className={s.name} title={name}>
                {name || (loadingHeader ? "Loading…" : "Unknown")}
                </span>
                <OnlineDot online={online} />
            </div>
            <div className={s.subRow}>{online ? "Online now" : "Offline"}</div>
            </div>
            <ChevronDown className={`${s.chevron} ${open ? s.open : ""}`} size={16} />
        </div>

                {headerActions && (
            <div
            className={s.headerActions}
            onClick={(e) => e.stopPropagation()} // to make sure a click won't toggle the header
            >
            {headerActions}
            </div>
        )}
      </div>

      {/* Collapsible panel */}
      <div id={panelId} className={s.panel} hidden={!open}>
        {loadingProfile && <div className={s.loading}>Loading profile…</div>}
        {!loadingProfile && profile && (
          <div className={s.profileBody}>
            {profile.about_me && (
              <p className={s.about}>{profile.about_me}</p>
            )}

            <div className={s.actionsRow}>
                <button
                    type="button"
                    className="u-btn"
                    onClick={() => (bio ? setBio(null) : handleLoadBio())}
                    aria-expanded={!!bio}
                    disabled={loadingBio}
                >
                    {loadingBio ? "Loading…" : bio ? "Hide biography" : "Show full biography"}
                </button>
                
            </div>

            {bio && (
                <section className={s.bio} aria-label="Full biography">
                    <BioRow label="Analog passions"  items={bio.analog_passions} />
                    <BioRow label="Digital delights" items={bio.digital_delights} />
                    <BioRow label="Seeking collaboration" items={bio.collaboration_interests} />
                    <BioRow label="Favorite food" items={bio.favorite_food} />
                    <BioRow label="Favorite music" items={bio.favorite_music} />
                </section>
            )}
          </div>
        )}
        {error && <div role="alert" className={s.error}>{error}</div>}
      </div>
    </article>
  );
};

export default CollapsibleUserCard;
