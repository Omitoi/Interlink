import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import api from "../api/axios";
import { useAuth } from "../context/AuthContext";
import s from "./ProfileWizard.module.css";
import Avatar from "../components/Avatar"
import { PictureRemover } from "../components/PictureUploader"
import LocationPicker from "../components/LocationPicker";

type Weights = {
  analog_passions: number;
  digital_delights: number;
  collaboration_interests: number;
  favorite_food: number;
  favorite_music: number;
  location: number;
};

type Step = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7;

const initialWeights: Weights = {
  analog_passions: 2,
  digital_delights: 2,
  collaboration_interests: 2,
  favorite_food: 1,
  favorite_music: 1,
  location: 5,
};

export default function ProfileWizard() {
  const navigate = useNavigate();

  const { user } = useAuth();
  const myId = user?.id;

  // Without the bustKey the browser cache causes the profile picture to not update
  // immediately. BustKey forces a cache refresh
  const [avatarBustKey, setAvatarBustKey] = useState(Date.now());


  const [step, setStep] = useState<Step>(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Step 0: Match Preferences
  const [weights, setWeights] = useState<Weights>(initialWeights);

  // Step 1: Bio
  const [displayName, setDisplayName] = useState("");
  const [aboutMe, setAboutMe] = useState("");

  // Step 2: Passions & Delights
  const [analogPassions, setAnalogPassions] = useState("");
  const [digitalDelights, setDigitalDelights] = useState("");

  // Step 3: Favorites
  const [favoriteFood, setFavoriteFood] = useState("");
  const [favoriteMusic, setFavoriteMusic] = useState("");

  // Step 4: Cross Pollination & Radius
  const [collaborationInterests, setCollaborationInterests] = useState("");
  const [maxRadiusKm, setMaxRadiusKm] = useState<number | "">(25);
  const [locationCity, setLocationCity] = useState("");
  const [locationLat, setLocationLat] = useState<number | "">("");
  const [locationLon, setLocationLon] = useState<number | "">("");

  // Step 5: Profile Picture
  const [selectedPictureFile, setSelectedPictureFile] = useState<File | null>(null);
  const [picturePreviewUrl, setPicturePreviewUrl] = useState<string | null>(null);
  const [uploadingPicture, setUploadingPicture] = useState(false);
  const [pictureError, setPictureError] = useState<string | null>(null);

  // Geolocation handler
  function handleUseMyLocation() {
    if (!("geolocation" in navigator)) return;
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLocationLat(Number(pos.coords.latitude.toFixed(6)));
        setLocationLon(Number(pos.coords.longitude.toFixed(6)));
      },
      () => {}
    );
  }

  // Picture upload handlers
  function handlePictureSelect(file: File | null) {
    setSelectedPictureFile(file);
    setPictureError(null);
    
    if (file) {
      // Create preview URL
      const previewUrl = URL.createObjectURL(file);
      setPicturePreviewUrl(previewUrl);
    } else {
      setPicturePreviewUrl(null);
    }
  }

  async function handleConfirmUpload() {
    if (!selectedPictureFile) return;
    
    setUploadingPicture(true);
    setPictureError(null);

    try {
      const formData = new FormData();
      formData.append("file", selectedPictureFile);

      await api.post("/me/avatar", formData);
      
      // Update avatar display
      setAvatarBustKey(Date.now());
      
      // Clear the selected file and preview after successful upload
      setSelectedPictureFile(null);
      setPicturePreviewUrl(null);
      
    } catch (err: unknown) {
      console.error(err);
      setPictureError("Upload failed. Please try again.");
    } finally {
      setUploadingPicture(false);
    }
  }

  const canNext = useMemo(() => {
    switch (step) {
      case 0: return true; // Introduction step
      case 1: return displayName.trim().length > 0 && aboutMe.trim().length > 0; // Bio
      case 2: return true; // Passions & Delights
      case 3: return true; // Favorites
      case 4: // Cross Pollination & Location
      return (
        typeof maxRadiusKm === "number" &&
        maxRadiusKm > 0 &&
        typeof locationLat === "number" &&
        typeof locationLon === "number"
      );
      case 5: return true; // Profile picture
      case 6: return true; // Match Preferences
      case 7: return true; // Congratulations
      default: return true;
    }
  }, [step, displayName, aboutMe, maxRadiusKm, locationLat, locationLon]);


  function setWeight(key: keyof Weights, val: number) {
    setWeights((w) => ({ ...w, [key]: Math.max(0, Math.min(10, val)) }));
  }

  function toList(input: string): string[] {
    return input.split(",").map((s) => s.trim()).filter(Boolean);
  }

  async function saveProfile() {
    setSaving(true);
    setError(null);
    try {
      const payload = {
        display_name: displayName.trim(),
        about_me: aboutMe.trim(),
        location_city: locationCity.trim() || null,
        location_lat: typeof locationLat === "number" ? locationLat : null,
        location_lon: typeof locationLon === "number" ? locationLon : null,
        max_radius_km: typeof maxRadiusKm === "number" ? maxRadiusKm : 25,
        analog_passions: toList(analogPassions),
        digital_delights: toList(digitalDelights),
        collaboration_interests: collaborationInterests.trim(),
        favorite_food: favoriteFood.trim(),
        favorite_music: favoriteMusic.trim(),
        other_bio: {},
        match_preferences: weights,
      };
      await api.post("/me/profile/complete", payload);
    } catch (err: unknown) {
      console.error(err);
      const error = err as { response?: { data?: { message?: string } } };
      const msg = error?.response?.data?.message || "Failed to complete profile.";
      setError(msg);
      throw err;
    } finally {
      setSaving(false);
    }
  }


  return (
    <div className={s.page}>
      <header className={s.header}>
        <h2 className={s.title}>Complete your profile</h2>
        <p className={s.progress}>Step {step + 1} of 8</p>
      </header>
      
      {error && <div className={s.error}>{error}</div>}

      {step === 0 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Welcome to Interlink!</h3>
          <div className={s.introContent}>
            <p className={s.introText}>
              <strong>"Your analog dreams, digitally connected."</strong>
            </p>
            <p className={s.introText}>
              Welcome to Interlink, a unique platform that connects people with fascinating hobbies 
              that blend the traditional and modern worlds. Whether you're into analog passions like 
              calligraphy or digital delights like retro gaming, we help you find others who share 
              your interests or complement your skills.
            </p>
            <p className={s.introText}>
              Our matching algorithm focuses on finding interesting connections and complementary 
              skill sets between your analog and digital hobbies. You might find a D&D group, 
              someone who codes and is passionate about blacksmithing, or a partner for competitive 
              knitting over Discord!
            </p>
            <p className={s.introText}>
              Let's get started by building your profile. This will take just a few minutes, 
              and you'll be able to update everything later.
            </p>
          </div>
        </section>
      )}

      {step === 1 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Bio</h3>
          <div className={s.fieldGrid}>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Display name *</span>
              <input 
                value={displayName} 
                onChange={(e) => setDisplayName(e.target.value)} 
                className={s.input}
                placeholder="Your name or preferred display name"
                required
              />
            </label>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>About me *</span>
              <textarea 
                value={aboutMe} 
                onChange={(e) => setAboutMe(e.target.value)} 
                className={`${s.input} ${s.textarea}`}
                placeholder="Tell others about yourself..."
                required
              />
            </label>
          </div>
        </section>
      )}

      {step === 2 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Passions & Delights</h3>
          <p className={s.stepHint}>Share your analog passions and digital delights to find like-minded people.</p>
          <div className={s.fieldGrid}>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Analog passions (comma-separated)</span>
              <input 
                value={analogPassions} 
                onChange={(e) => setAnalogPassions(e.target.value)} 
                className={s.input}
                placeholder="calligraphy, blacksmithing, woodworking"
              />
            </label>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Digital delights (comma-separated)</span>
              <input 
                value={digitalDelights} 
                onChange={(e) => setDigitalDelights(e.target.value)} 
                className={s.input}
                placeholder="retro gaming, digital art, coding"
              />
            </label>
          </div>
        </section>
      )}

      {step === 3 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Favorites</h3>
          <p className={s.stepHint}>Share your favorite food and music to connect with people who have similar tastes.</p>
          <div className={s.fieldGrid}>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Favorite food</span>
              <input 
                value={favoriteFood} 
                onChange={(e) => setFavoriteFood(e.target.value)} 
                className={s.input}
                placeholder="ramen, pizza, sushi"
              />
            </label>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Favorite music</span>
              <input 
                value={favoriteMusic} 
                onChange={(e) => setFavoriteMusic(e.target.value)} 
                className={s.input}
                placeholder="synthwave, jazz, indie rock"
              />
            </label>
          </div>
        </section>
      )}

      {step === 4 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Cross-pollination & Radius</h3>
          <p className={s.stepHint}>What kind of connections are you seeking, and how far are you willing to travel?</p>
          <div className={s.fieldGrid}>
            <label className={s.fieldRow}>
              <span className={s.fieldLabel}>Collaboration interests</span>
              <textarea 
                value={collaborationInterests} 
                onChange={(e) => setCollaborationInterests(e.target.value)} 
                className={`${s.input} ${s.textarea}`}
                placeholder="Want to collaborate on projects, learn new skills, find creative partners, join hobby groups..."
              />
            </label>
            <LocationPicker
              value={{
                city: locationCity,
                lat: locationLat,
                lon: locationLon,
                radiusKm: maxRadiusKm
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
          </div>
        </section>
      )}

            {step === 5 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Profile picture</h3>
          <p className={s.stepHint}>Upload a profile picture. Please use jpg format.</p>
          
          <div className={s.pictureSection}>
            {/* Current avatar or preview */}
            <div className={s.avatarContainer}>
              {picturePreviewUrl ? (
                <div>
                  <img 
                    src={picturePreviewUrl} 
                    alt="Picture preview" 
                    className={s.avatarPreview}
                    style={{ width: 256, height: 256, objectFit: 'cover', borderRadius: '8px' }}
                  />
                  <p className={s.previewLabel}>Preview (not yet uploaded)</p>
                </div>
              ) : (
                myId && <Avatar userId={myId} alt="Your current picture" size={256} bustKey={avatarBustKey}/>
              )}
            </div>

            {/* File input */}
            <div className={s.uploadControls}>
              <input
                type="file"
                accept="image/jpeg"
                onChange={(e) => handlePictureSelect(e.target.files?.[0] ?? null)}
                disabled={uploadingPicture}
                className={s.fileInput}
              />
              
              {/* Confirm upload button - only shown when file is selected */}
              {selectedPictureFile && (
                <button
                  type="button"
                  onClick={handleConfirmUpload}
                  disabled={uploadingPicture}
                  className={`u-btn u-btn--primary ${s.uploadButton}`}
                >
                  {uploadingPicture ? "Uploading…" : "Confirm Upload"}
                </button>
              )}
              
              {pictureError && (
                <div className={s.error} style={{ color: "red", marginTop: "8px" }}>
                  {pictureError}
                </div>
              )}
            </div>
          </div>

          {/* Keep the remover for removing existing pictures */}
          <PictureRemover onRemoved={() => {
            setAvatarBustKey(Date.now());
          }} />
        </section>
      )}

      {step === 6 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>Match Preferences</h3>
          <p className={s.stepHint}>Now that we know about you, adjust how much each category matters for finding your perfect match.</p>
          <div className={s.fieldGrid}>
            {(
              [
                ["analog_passions", "Analog passions"],
                ["digital_delights", "Digital delights"],
                ["collaboration_interests", "Collaboration interests"],
                ["favorite_food", "Favorite food"],
                ["favorite_music", "Favorite music"],
                ["location", "Location proximity"],
              ] as [keyof Weights, string][]
            ).map(([key, label]) => (
              <div key={key} className={s.weightRow}>
                <span className={s.weightLabel}>{label}</span>
                <input
                  type="range"
                  min={0}
                  max={10}
                  value={weights[key]}
                  onChange={(e) => setWeight(key, Number(e.target.value))}
                  className={s.weightSlider}
                />
                <span className={s.weightValue}>{weights[key]}/10</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {step === 7 && (
        <section className={`${s.stepCard} u-card`}>
          <h3 className={s.stepTitle}>🎉 Congratulations!</h3>
          <div className={s.introContent}>
            <p className={s.introText}>
              <strong>Your profile is now complete!</strong>
            </p>
            <p className={s.introText}>
              Your profile has been saved and you're ready to start discovering amazing connections. 
              Our algorithm will now match you with people who share 
              your interests or have complementary skills.
            </p>
            <p className={s.introText}>
              You can always update your profile later by visiting the Profile page. 
              Now let's see who's out there waiting to connect with you!
            </p>
          </div>
        </section>
      )}

      <nav className={s.navigation}>
        <button
          type="button"
          onClick={() => setStep((s) => (s > 0 ? ((s - 1) as Step) : s))}
          disabled={step === 0 || saving}
          className={`u-btn ${s.navButton}`}
        >
          Back
        </button>

        {step < 6 && (
          <button
            type="button"
            onClick={() => canNext && setStep((s) => ((s + 1) as Step))}
            disabled={!canNext || saving}
            className={`u-btn u-btn--primary ${s.navButton}`}
          >
            Next
          </button>
        )}

        {step === 6 && (
          <button
            type="button"
            onClick={async () => {
              if (!canNext || saving) return;
              try {
                await saveProfile(); 
                setStep(7);
              } catch {/* setError already done */}
            }}
            disabled={!canNext || saving}
            className={`u-btn u-btn--primary ${s.navButton}`}
          >
            {saving ? "Saving…" : "Save & continue"}
          </button>
        )}

        {step === 7 && (
          <button
            type="button"
            onClick={() => navigate("/recommendations")}   
            disabled={saving}
            className={`u-btn u-btn--primary ${s.navButton}`}
          >
            See My Recommendations
          </button>
        )}
      </nav>

    </div>
  );
}
