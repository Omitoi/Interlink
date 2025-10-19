import type React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

// This component checks for the JWT and either 
// - renders the page if logged in
// - redirects to / (login) if not

type Props = {
    children: React.ReactNode;
};

const ProtectedRoute = ({ children }: Props) => {
    const { token, user, loading } = useAuth();
    const location = useLocation();

    if (loading) return <div style={{ padding: 24 }}>Loading…</div>;

    if (!token || !user) {
        return <Navigate to="/" replace state={{ from: location }} />;
    }

    return <>{children}</>
};

export default ProtectedRoute;