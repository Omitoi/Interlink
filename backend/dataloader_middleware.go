package main

import (
	"database/sql"
	"net/http"
)

// DataLoaderMiddleware creates middleware that injects dataloaders into the request context
func DataLoaderMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create new dataloaders for each request to ensure freshness
			// In production, you might want to implement per-request caching differently
			dataloaders := NewDataLoaders(db)

			// Add dataloaders to request context
			ctx := WithDataLoaders(r.Context(), dataloaders)
			r = r.WithContext(ctx)

			// Continue with the request
			next.ServeHTTP(w, r)
		})
	}
}
