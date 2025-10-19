package main

import (
	"log"
	"net/http"
	"os"

	"gitea.kood.tech/petrkubec/match-me/backend/graph"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
)

// JWT secret from environment variable or fallback
func getJWTSecret() []byte {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return []byte(secret)
	}
	return []byte("your_secret_key_please_change_in_production")
}

var jwtSecret = getJWTSecret()

func main() {
	initDB()

	mux := http.NewServeMux()

	// Make sure that the upload directory for avatars exist
	_ = os.MkdirAll("./uploads/avatars", 0o755)

	// Core auth & user endpoints
	mux.Handle("/register", registerHandler(db))
	mux.Handle("/login", loginHandler(db))
	mux.Handle("/me", meHandler(db))
	mux.Handle("/me/profile", meProfileHandler(db))
	mux.Handle("/me/bio", meBioHandler(db))
	mux.Handle("/me/profile/complete", completeProfileHandler(db)) // POST/PATCH alias

	// Ping: mark this user as online "now"
	mux.Handle("/me/ping", mePingHandler(db)) // POST

	// Recommendations & connections
	mux.Handle("/recommendations", recommendationsHandler(db))
	mux.Handle("/recommendations/detailed", recommendationsDetailedHandler(db))
	mux.Handle("/recommendations/", dismissRecommendationHandler(db)) // /recommendations/{id}/dismiss
	mux.Handle("/connections", connectionsHandler(db))                // GET /connections
	mux.Handle("/connections/", connectionsActionsRouter(db))         // POST/DELETE /connections/{id}/...
	mux.Handle("/connections/requests", requestsHandler(db))          // Listing requested connections

	// Users dispatcher (summary, profile, bio)
	mux.Handle("/users/", usersDispatcher(db))

	// WebSocket chat endpoint
	mux.Handle("/ws/chat", wsChatHandler(db))

	// For fetching message history
	mux.Handle("/chats/", getChatHistoryHandler(db))

	// Chat summary for sidebar ordering + unread badge
	mux.Handle("/chats/summary", chatSummaryHandler(db)) // GET

	// Mark messages from peer as read in the active chat
	mux.Handle("/chats/read", chatsMarkReadHandler(db)) // POST /chats/read?peer_id=123

	mux.Handle("/me/avatar", myAvatarHandler(db))     // POST & DELETE
	mux.Handle("/avatars/", getUserAvatarHandler(db)) // GET /avatars/{id}

	// Health check endpoint for Docker
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// GraphQL endpoint with middleware chain (DataLoader + Auth)
	// Set JWT secret for GraphQL resolvers
	graph.SetJWTSecret(jwtSecret)
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: graph.NewResolver(db)}))

	// Create middleware chain: DataLoader -> Auth -> GraphQL
	graphqlHandler := graph.DataLoaderMiddleware(db)(graph.AuthMiddleware(srv))
	mux.Handle("/graphql", graphqlHandler)

	// GraphQL playground for development only
	goEnv := os.Getenv("GO_ENV")
	if goEnv == "development" || goEnv == "" {
		mux.Handle("/", playground.Handler("GraphQL playground", "/graphql"))
		log.Println("GraphQL playground available at http://localhost:8080/")
	} else {
		log.Println("GraphQL playground disabled in production mode")
	}
	
	log.Default().Println("Starting Match Me Backend on port 8080 (accessible via port 8081)...")
	http.ListenAndServe(":8080", withCORS(mux))
}
