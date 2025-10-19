import React from "react";
import { FI_CITIES } from "../data/fiCities";

export type LocationValue = {
  city: string;
  lat: number | "";
  lon: number | "";
  radiusKm?: number | "";
};

type Props = {
  value: LocationValue;
  onChange: (patch: Partial<LocationValue>) => void;
  showRadius?: boolean;           // for showing/hiding radius in Wizard or Profile
  onUseMyLocation?: () => void;   
  className?: string;
  
  labels?: {
    quickSelect?: string;
    customCity?: string;
    latitude?: string;
    longitude?: string;
    radius?: string;
    choose?: string;
    useMyLocation?: string;
  };
};

const defaultLabels = {
  quickSelect: "City (quick select)",
  customCity: "City (custom)",
  latitude: "Latitude",
  longitude: "Longitude",
  radius: "Max radius (km)",
  choose: "Choose…",
  useMyLocation: "Use my location",
};

export default function LocationPicker({
  value, onChange, showRadius = true, onUseMyLocation, className, labels = defaultLabels,
}: Props) {
  // On select, set city + lat/lon
  function handleCitySelect(e: React.ChangeEvent<HTMLSelectElement>) {
    const name = e.target.value;
    if (!name) return;
    const c = FI_CITIES.find(x => x.name === name);
    if (c) {
      onChange({
        city: c.name,
        lat: Number(c.lat.toFixed(6)),
        lon: Number(c.lon.toFixed(6)),
      });
    }
  }

  const isKnownCity = FI_CITIES.some(c => c.name === value.city);

  return (
    <div className={className}>
      {/* City quick select */}
      <label className="fieldRow">
        <div className="fieldLabel">{labels.quickSelect}</div>
        <select
          value={isKnownCity ? value.city : ""}
          onChange={handleCitySelect}
        >
          <option value="">{labels.choose}</option>
          {FI_CITIES.map(c => (
            <option key={c.name} value={c.name}>{c.name}</option>
          ))}
        </select>
      </label>

      {/* Custom city (free text) */}
      <label className="fieldRow">
        <div className="fieldLabel">{labels.customCity}</div>
        <input
          value={value.city}
          onChange={(e) => onChange({ city: e.target.value })}
          placeholder="Helsinki"
        />
      </label>

      {/* Use my location button (optional) */}
      {onUseMyLocation && (
        <button type="button" onClick={onUseMyLocation} className="u-btn u-btn--secondary" style={{ marginBottom: 8 }}>
          {labels.useMyLocation}
        </button>
      )}

      {/* Lat / Lon / Radius */}
      <div className="grid gridCols3">
        <label className="fieldRow">
          <div className="fieldLabel">{labels.latitude}</div>
          <input
            type="number"
            step="0.000001"
            value={value.lat}
            onChange={(e) => onChange({ lat: e.target.value === "" ? "" : Number(e.target.value) })}
            placeholder="60.1699"
          />
        </label>
        <label className="fieldRow">
          <div className="fieldLabel">{labels.longitude}</div>
          <input
            type="number"
            step="0.000001"
            value={value.lon}
            onChange={(e) => onChange({ lon: e.target.value === "" ? "" : Number(e.target.value) })}
            placeholder="24.9384"
          />
        </label>
        {showRadius && (
          <label className="fieldRow">
            <div className="fieldLabel">{labels.radius}</div>
            <input
              type="number"
              min={1}
              max={5000}
              value={typeof value.radiusKm === "number" ? value.radiusKm : ""}
              onChange={(e) => onChange({ radiusKm: e.target.value === "" ? "" : Number(e.target.value) })}
              placeholder="25"
            />
          </label>
        )}
      </div>
    </div>
  );
}
