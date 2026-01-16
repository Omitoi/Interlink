# Interlink

**A high-performance social graph engine and recommendation platform.**
Built with **Go**, **PostgreSQL**, and **TypeScript**. Fully dockerized.

---

## 🚀 Quick Start (Docker)

Get the entire stack (Backend + DB + Frontend) running in under a minute.

```bash
# Start all services (Postgres, Go API, React Frontend)
make up

# Seed the database with 100 fake users and connections
make seed
```

**Access:**
- **Frontend**: <http://localhost:3001> (Login: `user1@test.local` / `test1234`)
- **Backend API**: <http://localhost:8081>

---

## 🛠 Engineering Highlights

This project was built to demonstrate production-grade systems architecture, specifically focusing on **concurrency control**, **complex business logic**, and **reliability**.

### 1. Concurrency & Race Condition Handling
To prevent invalid states (e.g., dual request collisions) in the social graph, the backend uses **row-level locking** (`SELECT ... FOR UPDATE`) within atomic transactions.

**Key File**: [`backend/connections.go`](backend/connections.go)
```go
// From requestConnectionHandler:
// A transaction ensures atomicity, while 'loadPairForUpdate' locks the specific connection row.
// This prevents race conditions when two users request each other simultaneously.
err = withTx(r.Context(), db, func(tx *sql.Tx) error {
    row, err := loadPairForUpdate(tx, me, targetID)
    // ... mutual request auto-acceptance logic ...
})
```

### 2. O(n) Semantic Matching Algorithm
The recommendation engine uses a sophisticated weighted scoring system that matches users based on 6 dimensions (including "Analog Passions" vs "Digital Delights"). It performs semantic groupings (e.g., "Piano" matches "Music") without external ML dependencies.

**Key File**: [`backend/helper.go`](backend/helper.go)
- See `calculateInterestScore` for the O(n) semantic matching logic.

---

## ⚡ Tech Stack

- **Backend**: Go (Golang) 1.22+
    - Standard Library (`net/http`) for high performance.
    - `lib/pq` for PostgreSQL connectivity.
- **Frontend**: React + TypeScript + Vite
    - Modern component architecture.
    - Real-time WebSockets for chat.
- **Data**: PostgreSQL 16
    - Complex relational schema for social graph.
- **Infrastructure**: Docker & Docker Compose
    - Multi-stage builds for small image sizes.
    - 12-factor app configuration via `DATABASE_URL`.

---

## 📁 Project Structure

```text
match-me/
├── backend/            # Go Standard Lib API
│   ├── connections.go  # Core social graph logic (The "Proud" Code)
│   ├── helper.go       # Matching algorithm
│   └── cmd/            # Entry points
├── frontend/           # React/Vite TypeScript App
├── db-seeder/          # Go tool to generate 10k+ fake users
├── docker-compose.yml  # Production orchestration
└── Makefile            # Developer convenience scripts
```

---

## ✅ Quality Assurance

The critical paths (Auth, Connections, Matching) are covered by a comprehensive test suite.

```bash
# Run backend tests
cd backend
go test ./...
```
*Note: `connections_test.go` alone is 53KB, covering every state transition edge case.*
