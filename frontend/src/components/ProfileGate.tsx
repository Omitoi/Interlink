import React from "react";
import { Navigate, useLocation } from "react-router-dom";
import useMyProfile from "../hooks/useMyProfile";

type Props = { children: React.ReactNode };

const ProfileGate: React.FC<Props> = ({ children }) => {
  const { loading, error, isComplete } = useMyProfile();
  const location = useLocation();

  if (loading) return <div style={{ padding: 24 }}>Checking your profile…</div>;

  // If request errored (not 404), fail softly — let the page render
  if (error) return <div style={{ padding: 24, color: "#b00" }}>{error}</div>;

  if (isComplete === false) {
    // Redirect to /profile to complete it
    return (
      <Navigate
        to="/profile/wizard"
        replace
        state={{ from: location, reason: "incomplete-profile" }}
      />
    );
  }

  // isComplete === true
  return <>{children}</>;
};

export default ProfileGate;
