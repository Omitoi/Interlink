// Here we have a one place to ask 'is my profile complete?'.
// This is by calling GET /me/profile
// The backend returns 404 on a missing profile
// 200 is treated as complete, 404 as incomplete

import { useEffect, useRef, useState } from "react";
import api from "../api/axios";
import { MeProfileResponse } from "../types/api";

type UseMyProfileState = {
  loading: boolean;
  error: string | null;
  profile: MeProfileResponse | null;
  isComplete: boolean | null; // null while loading
  refetch: () => void;
};

export default function useMyProfile(): UseMyProfileState {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [profile, setProfile] = useState<MeProfileResponse | null>(null);
  const [isComplete, setIsComplete] = useState<boolean | null>(null);
  const tick = useRef(0);

  const fetchProfile = async () => {
    const myTick = ++tick.current;
    setLoading(true);
    setError(null);
    try {
      const { data } = await api.get<MeProfileResponse>("/me/profile");
      if (myTick !== tick.current) return; // prevent stale setState
      setProfile(data);
      setIsComplete(true);
    } catch (err: unknown) {
      if (myTick !== tick.current) return;
      const error = err as { response?: { status?: number; data?: { message?: string } } };
      if (error?.response?.status === 404) {
        setProfile(null);
        setIsComplete(false);
        setError(null);
      } else {
        setError(error?.response?.data?.message || "Failed to load profile.");
        setIsComplete(null);
      }
    } finally {
      if (myTick === tick.current) setLoading(false);
    }
  };

  useEffect(() => {
    fetchProfile();
    // no deps: we fetch once on mount; caller can call refetch()
  }, []);

  return { loading, error, profile, isComplete, refetch: fetchProfile };
}
