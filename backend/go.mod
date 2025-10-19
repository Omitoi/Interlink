module gitea.kood.tech/petrkubec/match-me/backend

go 1.25

require (
	// GraphQL
	github.com/99designs/gqlgen v0.17.81

	// Authentication & Security
	github.com/golang-jwt/jwt/v5 v5.3.0

	// WebSocket
	github.com/gorilla/websocket v1.5.3

	// DataLoader
	github.com/graph-gophers/dataloader/v7 v7.1.2

	// Database
	github.com/lib/pq v1.10.9

	// Testing
	github.com/stretchr/testify v1.11.1
	github.com/vektah/gqlparser/v2 v2.5.30
	golang.org/x/crypto v0.42.0
)

require (
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
