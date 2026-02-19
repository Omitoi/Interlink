package main

import (
	"net/http"
	"os"
	"strings"
)

// allowedOrigins is built once at startup from the CORS_ALLOWED_ORIGINS env var.
// Fallback to common dev origins if the env var is not set.
var allowedOrigins = buildAllowedOrigins()

func buildAllowedOrigins() map[string]struct{} {
	origins := make(map[string]struct{})
	env := os.Getenv("CORS_ALLOWED_ORIGINS")
	if env != "" {
		for _, o := range strings.Split(env, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				origins[o] = struct{}{}
			}
		}
		return origins
	}
	// Fallback defaults for backward compatibility
	for _, o := range []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:3001",
		"http://127.0.0.1:3001",
	} {
		origins[o] = struct{}{}
	}
	return origins
}

// The backend needs Cross-Origin Resource Sharing to function with the frontend in modern browsers.
// The CORS headers need to be set here in order to make the backend available to the frontend.

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
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
