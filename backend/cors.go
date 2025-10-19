package main

import "net/http"

// The backend needs Cross-Origin Reasource Sharing to function with the frontend in modern browsers.
// The CORS headers need to be set here in order to make the backend available to the frontend.

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from the frontend dev server and Docker frontend
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" ||
			origin == "http://localhost:3001" || origin == "http://127.0.0.1:3001" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			// default to localhost:3001 for Docker frontend
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3001")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
