// components/Avatar.tsx
import React from "react";
import { fetchAvatarBlob } from "../api/avatars";
import AvatarPlaceholder from "../assets/avatar-placeholder.png"

type Props = {
  userId: number;
  alt: string;
  size?: number;
  bustKey?: string | number;
  fallbackSrc?: string;
  className?: string;
};

export default function Avatar({ userId, alt, size = 64, bustKey, fallbackSrc = AvatarPlaceholder, className }: Props) {
  const [src, setSrc] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!Number.isFinite(userId)) return;
    let alive = true;
    let objectUrl: string | null = null;

    fetchAvatarBlob(userId)
      .then((blob) => {
        if (!alive) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch(() => {
        if (!alive) return;
        setSrc(fallbackSrc);
      });

    return () => {
      alive = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [userId, bustKey, fallbackSrc]);

  return (
    <img
      src={src ?? fallbackSrc}
      alt={alt}
      width={size}
      height={size}
      className={className}
      style={{ objectFit: "cover", borderRadius: "8px" }}
    />
  );
}
