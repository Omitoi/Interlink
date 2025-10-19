import React, { useState, useEffect, useRef } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import api from "../api/axios";
import s from "./Profile.module.css";
import Avatar from "../components/Avatar"
import { PictureUploader, PictureRemover } from "../components/PictureUploader";
import LocationPicker from "../components/LocationPicker";
import { useAuth } from "../context/AuthContext";
import { useToast } from "../hooks/useToast";
import ToastContainer from "../components/ToastContainer";


type SaveState = "idle" | "saving" | "error" | "success";

type LocationState = {
  reason?: string;
  from?: {
    pathname: string;
  };
};

type ApiError = {
  response?: {
    status?: number;
    data?: {
      message?: string;
    };
  };
  message?: string;
};

type CardProps = {
  title: string;
  children?: React.ReactNode;
};

function Card({ title, children }: CardProps) {
  return (
    <section className={`s.card u-card`}>
      <div className={s.cardHeader}>
        <h3 className={s.cardTitle}>{title}</h3>
      </div>
      <div className={s.cardContent}>{children}</div>
    </section>
  );
}

type FieldRowProps = {
  label: string;
  children?: React.ReactNode;
}

function FieldRow({ label, children }: FieldRowProps) {
  return (
    <label className={s.fieldRow}>
      <div className={s.fieldLabel}>{label}</div>
      {children}
    </label>
  );
}

type ReadOnlyFieldProps = {
  label: string;
  value?: string | string[] | number;
  placeholder?: string;
}

function ReadOnlyField({ label, value, placeholder = "Not set" }: ReadOnlyFieldProps) {
  const displayValue = Array.isArray(value) ? value.join(", ") : value;
  
  return (
    <div className={s.fieldRow}>
      <div className={s.fieldLabel}>{label}</div>
      <div className={s.readOnlyValue}>
        {displayValue || <span className={s.placeholder}>{placeholder}</span>}
      </div>
    </div>
  );
}

const Profile: React.FC = () => {
  const location = useLocation() as { state?: LocationState };
  const navigate = useNavigate();
  const { logout } = useAuth();
  const { toasts, success } = useToast();
  const reason = location.state?.reason;
  
  function handleLogout() {
    logout();
    navigate("/");
  }

  // Without the bustKey the browser cache causes the profile picture to not update
  // immediately. BustKey forces a cache refresh
  const [avatarBustKey, setAvatarBustKey] = useState(Date.now());

  // --- form state ---
  const [userId, setUserId] = useState<number>(NaN);
  const [displayName, setDisplayName] = useState("");
  const [aboutMe, setAboutMe] = useState("");
  const [locationCity, setLocationCity] = useState("");
  const [locationLat, setLocationLat] = useState<number | ''>('');
  const [locationLon, setLocationLon] = useState<number | ''>('');
  const [maxRadiusKm, setMaxRadiusKm] = useState<number | ''>(25);

  // comma-separated inputs
  const [analogPassions, setAnalogPassions] = useState("");
  const [digitalDelights, setDigitalDelights] = useState("");

  // free-text
  const [collaborationInterests, setCollaborationInterests] = useState("");
  const [favoriteFood, setFavoriteFood] = useState("");
  const [favoriteMusic, setFavoriteMusic] = useState("");

  // JSON textareas
  const [otherBio, setOtherBio] = useState("");

  const [status, setStatus] = useState<SaveState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  // Prefilling the fields with existing data
  // Prevent overwriting user edits once we prefill
  const prefilled = useRef(false);

  type Weights = {
    analog_passions: number;
    digital_delights: number;
    collaboration_interests: number;
    favorite_food: number;
    favorite_music: number;
    location: number;
  };

  const [weights, setWeights] = useState<Weights>({
    analog_passions: 2,
    digital_delights: 2,
    collaboration_interests: 2,
    favorite_food: 1,
    favorite_music: 1,
    location: 5,
  });

  // Helper to update a single weight

  function setWeight(key: keyof Weights, val: number) {
    setWeights((w) => ({ ...w, [key]: Math.max(0, Math.min(10, val)) }));
  }

  useEffect(() => {
    let alive = true;

    async function loadExisting() {
      try {
        if (!alive) return;
        if (prefilled.current) return;

        const res = await api.get("/me/profile");
        setUserId(typeof res.data.id === "number" ? res.data.id : null);

        const isPristine =
          displayName === "" &&
          aboutMe === ""

        if (isPristine) {
          setDisplayName(res.data.display_name ?? "");
          setAboutMe(res.data.about_me ?? "");
          setLocationCity(res.data.location_city ?? "");
          setLocationLat(typeof res.data.location_lat === "number" ? res.data.location_lat : "");
          setLocationLon(typeof res.data.location_lon === "number" ? res.data.location_lon : "");
          setMaxRadiusKm(typeof res.data.max_radius_km === "number" ? res.data.max_radius_km : 25);
          setAnalogPassions(Array.isArray(res.data.analog_passions) ? res.data.analog_passions.join(", ") : "");
          setDigitalDelights(Array.isArray(res.data.digital_delights) ? res.data.digital_delights.join(", ") : "");
          setCollaborationInterests(res.data.collaboration_interests ?? "");
          setFavoriteFood(res.data.favorite_food ?? "");
          setFavoriteMusic(res.data.favorite_music ?? "");
          setOtherBio(res.data.other_bio ? JSON.stringify(res.data.other_bio) : "");
          if (res.data.match_preferences) {
            setWeights(w => ({ ...w, ...res.data.match_preferences }));
          }

          prefilled.current = true;
        }
      } catch (err: unknown) {
        const apiError = err as ApiError;
        if (apiError?.response?.status === 404) {
          // No profile yet, leave fields empty
          return;
        }
        setError(apiError?.response?.data?.message || "Failed to load existing profile.");
      }
    }


    loadExisting();
    return () => {
      alive = false;
    };
    // We want to run this once on mount; we purposely don't include form fields in deps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);  
  // The empty dependecy array here causes useEffect run only once on mount.
  // E.g. We read variables like displayName, aboutMe and profilePictureFile. If the effect
  // was run every time they change, it would overwrite the user's changes.

  // Helper functions
  function toList(input: string): string[] {
    return input
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  }

  function parseJSONOrEmpty(input: string): Record<string, unknown> {
    if (!input.trim()) return mapEmpty();
    try {
      return JSON.parse(input);
    } catch {
      return mapEmpty();
    }
  }

  function mapEmpty(): Record<string, unknown> {
    return {};
  }

  // User location data
  async function handleUseMyLocation() {
    if (!("geolocation" in navigator)) {
      setError("Geolocation not available in this browser.");
      return;
    }
    setError(null);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLocationLat(Number(pos.coords.latitude.toFixed(6)));
        setLocationLon(Number(pos.coords.longitude.toFixed(6)));
      },
      (err) => {
        setError("Failed to get location: " + err.message);
      },
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 }
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setStatus("saving");
    setError(null);

    // basic validation: require displayName and aboutMe
    if (!displayName.trim() || !aboutMe.trim()) {
      setStatus("error");
      setError("Display name and About me are required.");
      return;
    }

    const payload = {
      display_name: displayName.trim(),
      about_me: aboutMe.trim(),
      location_city: locationCity.trim(),
      location_lat: typeof locationLat === "number" ? locationLat : 0,
      location_lon: typeof locationLon === "number" ? locationLon : 0,
      max_radius_km: typeof maxRadiusKm === "number" ? maxRadiusKm : 25,
      analog_passions: toList(analogPassions),
      digital_delights: toList(digitalDelights),
      collaboration_interests: collaborationInterests.trim(),
      favorite_food: favoriteFood.trim(),
      favorite_music: favoriteMusic.trim(),
      other_bio: parseJSONOrEmpty(otherBio),
      match_preferences: weights,
    };

    try {
      await api.post("/me/profile/complete", payload);
      setStatus("success");
      success(
        "Profile saved",
        "Changes in your profile have been saved",
      )

      setIsEditing(false);

    } catch (err: unknown) {
      console.error(err);
      const apiError = err as ApiError;
      setStatus("error");
      setError(apiError?.response?.data?.message || "Failed to save profile.");
    }
  }

return (
    <div className={s.page}>
      {reason === "incomplete-profile" && (
        <div className={s.notice}>
          Please complete your profile to access recommendations and chat.
        </div>
      )}

      <div className={s.header}>
        <h2>Your Profile</h2>
        <div className={s.headerActions}>
          <button 
            type="button" 
            onClick={() => setIsEditing(!isEditing)} 
            className="u-btn u-btn--secondary"
          >
            {isEditing ? "Cancel" : "Edit Profile"}
          </button>
          <button 
            type="button" 
            onClick={handleLogout} 
            className="u-btn u-btn--danger"
          >
            <LogOut size={18} /><span className={s.buttonText}>Logout</span>
          </button>
        </div>
      </div>

      <p className={s.lead}>
        {isEditing 
          ? "Interlink matches you by blending your analog passions and digital delights."
          : "Your profile information helps us connect you with like-minded people."
        }
      </p>

      {error && <div className={s.error}>{error}</div>}
      
      {/* Combined profile header and content - no gap */}
      {Number.isFinite(userId) && (
        <div className={s.profileContainer}>
          <div className={s.profileHeader}>
            <div className={s.profilePictureContainer}>
              <Avatar userId={userId!} alt="User profile image" size={400} bustKey={avatarBustKey} />
              
              {/* Status overlay - top left */}
              <div className={s.statusOverlay}>
                <span className={s.dotOnline} aria-label="online" />
                <span>Online</span>
              </div>
              
              {/* Name overlay - bottom */}
              <div className={s.nameOverlay}>
                <h1 className={s.profileName}>
                  {displayName || "Your Profile"}
                </h1>
              </div>
            </div>
            
            {/* Edit controls moved under the profile picture */}
            {isEditing && (
              <div className={s.editControls}>
                <PictureUploader 
                  onUploaded={() => setAvatarBustKey(Date.now())}
                />
                <PictureRemover 
                  onRemoved={() => setAvatarBustKey(Date.now())}
                />
              </div>
            )}
          </div>
          
          {/* Content section */}
          <div className={s.contentSection}>

      {isEditing ? (
        <form onSubmit={handleSubmit} className={s.form}>
          <Card title="Basics">
            <div className={s.grid}>
              <FieldRow label="Display name *">
                <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} required />
              </FieldRow>
              <FieldRow label="About me *">
                <textarea rows={4} value={aboutMe} onChange={(e) => setAboutMe(e.target.value)} required />
              </FieldRow>
            </div>
          </Card>

          <Card title="Location">
            <LocationPicker
              value={{
                city: locationCity,
                lat: locationLat,
                lon: locationLon,
                radiusKm: maxRadiusKm,
              }}
              onChange={(patch) => {
                if (patch.city !== undefined) setLocationCity(patch.city);
                if (patch.lat !== undefined) setLocationLat(patch.lat);
                if (patch.lon !== undefined) setLocationLon(patch.lon);
                if (patch.radiusKm !== undefined) setMaxRadiusKm(patch.radiusKm);
              }}
              showRadius={true}
              onUseMyLocation={handleUseMyLocation}
            />
          </Card>



          <Card title="Interests">
            <div className={s.grid}>
              <FieldRow label="Analog passions (comma-separated)">
                <input
                  value={analogPassions}
                  onChange={(e) => setAnalogPassions(e.target.value)}
                  placeholder="calligraphy, blacksmithing"
                />
              </FieldRow>
              <FieldRow label="Digital delights (comma-separated)">
                <input
                  value={digitalDelights}
                  onChange={(e) => setDigitalDelights(e.target.value)}
                  placeholder="retro gaming, digital art"
                />
              </FieldRow>
              <FieldRow label="Cross-pollination goal">
                <input
                  value={collaborationInterests}
                  onChange={(e) => setCollaborationInterests(e.target.value)}
                  placeholder="create an app for blacksmiths together"
                />
              </FieldRow>
            </div>
          </Card>

          <Card title="Tastes">
            <div className={`${s.grid} ${s.gridCols2}`}>
              <FieldRow label="Favorite food">
                <input
                  value={favoriteFood}
                  onChange={(e) => setFavoriteFood(e.target.value)}
                  placeholder="ramen"
                />
              </FieldRow>
              <FieldRow label="Favorite music">
                <input
                  value={favoriteMusic}
                  onChange={(e) => setFavoriteMusic(e.target.value)}
                  placeholder="synthwave"
                />
              </FieldRow>
            </div>
          </Card>

          
          <Card title="Match weights (How important is to match on this?)">
            <div className={s.grid}>
              {([
                ["analog_passions", "Analog passions"] as const,
                ["digital_delights", "Digital delights"] as const,
                ["collaboration_interests", "Collaboration interests"] as const,
                ["favorite_food", "Favorite food"] as const,
                ["favorite_music", "Favorite music"] as const,
                ["location", "Location proximity (0 turns off location restrictions)"] as const,
              ]).map(([key, label]) => (
                <div key={key} className={s.weightRow}>
                  <span className={s.weightLabel}>{label}</span>
                  <input
                    type="range"
                    min={0}
                    max={10}
                    step={1}
                    value={weights[key]}
                    onChange={(e) => setWeight(key, Number(e.target.value))}
                    className={s.weightSlider}
                  />
                  <span className={s.weightValue}>{weights[key]}/10</span>
                </div>
              ))}
            </div>
          </Card>


          <div className={s.actions}>
            <button type="submit" className="u-btn u-btn--primary" disabled={status === "saving"}>
              {status === "saving" ? "Saving…" : "Save profile"}
            </button>
            <button type="button" onClick={() => setIsEditing(false)} className="u-btn" disabled={status === "saving"}>
              Cancel
            </button>
          </div>
        </form>
      ) : (
        <div className={s.readOnlyView}>
          <Card title="Basics">
            <div className={s.grid}>
              <ReadOnlyField label="Display name" value={displayName} />
              <ReadOnlyField label="About me" value={aboutMe} />
            </div>
          </Card>

          <Card title="Location">
            <div className={s.grid}>
              <ReadOnlyField label="City" value={locationCity} />
              <div className={`${s.grid} ${s.gridCols3}`}>
                <ReadOnlyField label="Latitude" value={locationLat} />
                <ReadOnlyField label="Longitude" value={locationLon} />
                <ReadOnlyField label="Max radius (km)" value={maxRadiusKm} />
              </div>
            </div>
          </Card>

          <Card title="Interests">
            <div className={s.grid}>
              <ReadOnlyField 
                label="Analog passions" 
                value={analogPassions ? toList(analogPassions) : []} 
              />
              <ReadOnlyField 
                label="Digital delights" 
                value={digitalDelights ? toList(digitalDelights) : []} 
              />
              <ReadOnlyField label="Collaboration interests" value={collaborationInterests} />
            </div>
          </Card>

          <Card title="Tastes">
            <div className={`${s.grid} ${s.gridCols2}`}>
              <ReadOnlyField label="Favorite food" value={favoriteFood} />
              <ReadOnlyField label="Favorite music" value={favoriteMusic} />
            </div>
          </Card>

          <Card title="Match preferences">
            <div className={s.grid}>
              {([
                ["analog_passions", "Analog passions"] as const,
                ["digital_delights", "Digital delights"] as const,
                ["collaboration_interests", "Collaboration interests"] as const,
                ["favorite_food", "Favorite food"] as const,
                ["favorite_music", "Favorite music"] as const,
                ["location", "Location proximity"] as const,
              ]).map(([key, label]) => (
                <div key={key} className={s.weightDisplay}>
                  <span className={s.fieldLabel}>{label}</span>
                  <div className={s.weightValue}>
                    <div className={s.weightBar}>
                      <div 
                        className={s.weightFill} 
                        style={{ width: `${(weights[key] / 10) * 100}%` }}
                      />
                    </div>
                    <span className={s.weightNumber}>{weights[key]}/10</span>
                  </div>
                </div>
              ))}
            </div>
          </Card>

        </div>
      )}
          </div>
        </div>
      )}
      <ToastContainer toasts={toasts} />
    </div>
  );

};

export default Profile;
