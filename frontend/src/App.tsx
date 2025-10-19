import { Routes, Route, useLocation } from 'react-router-dom';
import Login from './pages/Login';
import Signup from './pages/Signup';
import Dashboard from './pages/Dashboard';
import Profile from './pages/Profile';
import ProfileWizard from './pages/ProfileWizard';
import Chat from './pages/Chat';
import Recommendations from './pages/Recommendations';
import NotFound from './pages/NotFound';
import Navbar from './components/Navbar';
import ProtectedRoute from './components/ProtectedRoute';
import ProfileGate from './components/ProfileGate';

// PAUNO 11.8.25: Everything except the signup and login pages are protected.
// Protection means that they are accessible only with a valid token.
// Most of the pages require the user profile to be completed before they
// have any meaningful functionality. That's why they are gated.

function App() {
  const location = useLocation()
  const hideNavbarPaths = ['/', '/signup']
  const shouldHideNavbar = hideNavbarPaths.includes(location.pathname)

 return (
    <>
      {!shouldHideNavbar && <Navbar />}
      <Routes>
        <Route path="/" element={<Login />} />
        <Route path="/signup" element={<Signup />} />

        {/* Protected but NOT gated — must be able to complete profile */}
        <Route
          path="/profile"
          element={
            <ProtectedRoute>
              <Profile />
            </ProtectedRoute>
          }
        />

        {/* Wizard version */}
        <Route
          path="/profile/wizard"
          element={
            <ProtectedRoute>
              <ProfileWizard />
            </ProtectedRoute>
          }
        />

        {/* Protected AND gated */}
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <ProfileGate>
                <Dashboard />
              </ProfileGate>
            </ProtectedRoute>
          }
        />

        {/* ? makes peerId optional - route also works without parameters */}
        <Route
          path="/chat/:peerId?" 
          element={
            <ProtectedRoute>
              <ProfileGate>
                <Chat />
              </ProfileGate>
            </ProtectedRoute>
          }
        />

        <Route
          path="/recommendations"
          element={
            <ProtectedRoute>
              <ProfileGate>
                <Recommendations />
              </ProfileGate>
            </ProtectedRoute>
          }
        />

        <Route path="*" element={<NotFound />} />
      </Routes>
    </>
  );
}

export default App
