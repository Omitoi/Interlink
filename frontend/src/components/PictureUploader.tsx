// src/components/PictureUploader.tsx
import React, { useState, useRef } from "react";
import { Upload, X } from "lucide-react";
import api from "../api/axios";

type Props = {
  onUploaded?: () => void;
  onRemoved?: () => void;
};

export const PictureRemover: React.FC<Props> = ({ onRemoved }) => {
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleRemove() {
    setRemoving(true);
    setError(null);

    try {
      await api.delete("/me/avatar");

      if (onRemoved) onRemoved();
    } catch (err: unknown) {
      console.error(err);
      setError("Removing picture failed. Please try again.");
    } finally {
      setRemoving(false);
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={handleRemove}
        disabled={removing}
        className="u-btn u-btn--danger"
      >
        <X size={18} />
        {removing ? "Removing..." : "Remove"}
      </button>

      {error && (
        <div className="file-error">
          {error}
        </div>
      )}
    </>
  );
};

export const PictureUploader: React.FC<Props> = ({ onUploaded}) => {
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function handleUpload() {
    if (!file) return;
    setUploading(true);
    setError(null);
    setSuccess(false);

    try {
      const formData = new FormData();
      formData.append("file", file);

      await api.post("/me/avatar", formData);

      if (onUploaded) onUploaded();
      setSuccess(true);
      // Clear file and success after a delay
      setTimeout(() => {
        setFile(null);
        setSuccess(false);
      }, 2000);
    } catch (err: unknown) {
      console.error(err);
      setError("Upload failed. Please try again.");
    } finally {
      setUploading(false);
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const selectedFile = e.target.files?.[0] ?? null;
    setFile(selectedFile);
    setError(null);
    setSuccess(false);
  }

  function triggerFileSelect() {
    fileInputRef.current?.click();
  }

  return (
    <>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg"
        onChange={handleFileSelect}
        disabled={uploading}
        style={{ display: "none" }}
      />
      <button
        type="button"
        onClick={triggerFileSelect}
        disabled={uploading}
        className="u-btn"
      >
        <Upload size={18} />
        {file ? "Change" : "Select"}
      </button>
      {file && (
        <button
          type="button"
          onClick={handleUpload}
          disabled={uploading || success}
          className="u-btn u-btn--primary"
          style={success ? { background: 'var(--color-online)', borderColor: 'var(--color-online)' } : undefined}
        >
          {uploading ? "Uploading…" : success ? "✓ Uploaded" : "Upload"}
        </button>
      )}

      {/* File preview and errors - will be positioned by parent flex */}
      {file && (
        <div className="file-preview">
          Selected: {file.name}
        </div>
      )}

      {error && (
        <div className="file-error">
          {error}
        </div>
      )}
    </>
  );
};

