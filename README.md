# Interlink

**"Your analog dreams, digitally connected"**
Interlink is a full-stack recommendation application designed to connect users based on their profile information. This project utilizes Go for the backend server, TypeScript for the frontend, and PostgreSQL for data persistence.

- **Backend:** Go (REST API + PostgreSQL)  
- **Frontend:** React + TypeScript + Vite  
- **Database:** PostgreSQL  
- **Other:** WebSockets for chat & online status, database seeder for test data  

This README is the **starting point**: how to run the code, how the project is organized, and where to find the seeder.  

---

## Easiest way to get started is to run the project in the provided Docker container

Please see DOCKER_README.md for detailed instructions

## Alternatively, run the project locally following these instructions

### 1. Requirements

Make sure you have installed:

- [Go](https://go.dev/dl/) (≥ 1.22)  
- [Node.js](https://nodejs.org/) (≥ 20) + npm  
- [Docker](https://docs.docker.com/get-docker/)

### 2. Clone and enter the project

```bash
git clone https://gitea.kood.tech/petrkubec/match-me
cd match-me
```

### 3. Start the database (Postgres)

```bash
docker compose up -d postgres
```

This creates a local Postgres container. Next set the database url

```bash
export DATABASE_URL="postgresql://matchme_user:matchme_password@localhost:5432/matchme_db?sslmode=disable"
```

### 4. Run the backend (Go API)

```bash
cd backend
go run .
```

The backend runs on <http://localhost:8080> (for local development).  
*Note: When using Docker, the backend runs on <http://localhost:8081>*  

### 5. Run the frontend (React/Vite)

Open a new terminal:

```bash
cd frontend
npm install       # install dependencies
npm run dev       # start local dev server
```

The frontend runs on <http://localhost:5173> (for local development).  
*Note: When using Docker, the frontend runs on <http://localhost:3001>*  

---

## Project Structure

```text
match-me/
│  README.md            ← you are here
│  DOCKER_README.md     ← docker-specific notes
│  QUICK_REFERENCE.md   ← short cheatsheet for devs
│
├─ backend/             ← Go backend (API, auth, recommendations, chat)
│    └─ README.md
│
├─ frontend/            ← React + Vite frontend
│    └─ README.md
│
├─ db/                  ← Database schema + migrations
│
├─ db-seeder/           ← Seeder tool for fake users/connections
│    └─ README.md
│
└─ docs/                ← API spec + developer docs
```

---

## Database Seeder

For testing and demos, you can generate **fake users, profiles, and connections**.  
Seeder lives in **`db-seeder/`**.  

### Run Seeder (example)

```bash
cd db-seeder
go run main.go -count=50 -truncate
```

- `-count=50` → create 50 users  
- `-truncate` → wipe old data before seeding (recommended for clean testing)  
- Other flags let you control ratios of accepted/pending/declined connections.

**Note:** The `make seed` and `make seed-dev` commands automatically include `-truncate` for convenience.  

See [`db-seeder/README.md`](./db-seeder/README.md) for full instructions.  

---

## More Docs

- [Backend README](./backend/README.md) – API server, auth, recommendations  
- [Frontend README](./frontend/README.md) – how React/Vite app works  
- [Docs folder](./docs/README.md) – API spec and architecture notes  

---

## Notes for Reviewers

- The **frontend** is built with React/Vite. If you haven’t used these before, don’t worry — running `npm install && npm run dev` is enough.  
- The **backend** is plain Go, no frameworks. Main entrypoint is `backend/`.  
- Everything is wired together through **PostgreSQL**. Docker handles this, no manual DB install needed.  
- The **seeder** helps you skip manual sign-ups and instantly test recommendations/chat.  

## Advanced Matchmaking System

**Match-Me** features a sophisticated multi-dimensional compatibility engine that analyzes user profiles across six key dimensions to find meaningful connections.

### System Overview

The recommendation system uses **weighted compatibility scoring** to match users based on:

- **Analog Passions** (creative/physical activities)
- **Digital Delights** (online/tech interests)  
- **Collaboration Interests** (activities to do together)
- **Food Preferences** (cuisine compatibility)
- **Music Taste** (genre preferences)
- **Geographic Proximity** (location-based matching)

### Recommendation Flow

#### 1. API Endpoints

- **`GET /recommendations`** → Returns up to 10 user IDs ranked by compatibility
- **`GET /recommendations/detailed`** → Returns full recommendation data with scores and percentages
- **Authentication Required:** JWT token validation
- **Profile Gating:** Only users with complete profiles can receive recommendations

#### 2. Candidate Filtering Pipeline

**Inclusion Criteria:**

- Complete profiles only (`is_complete = TRUE`)
- Not the requesting user
- Within set radius limit (when radius > 0)

**Exclusion Criteria:**  

- Existing connections (any status)
- Previously dismissed recommendations
- Users outside geographic radius (hard cutoff)

### Multi-Dimensional Scoring Algorithm

#### 1. **Analog Passions** (Creative/Physical Activities)

- **Exact Matches:** 3 points each
- **Semantic Matches:** 1 point each (grouped by themes)
- **High Overlap Bonus:** +5 points (>50% shared interests)
- **Semantic Groups:** music, visual arts, tech, crafts, games, outdoor, food, fitness
- **Final Score:** `(base_score × user_weight) ÷ 3`

#### 2. **Digital Delights** (Online/Tech Interests)

- Same scoring logic as analog passions
- **Examples:** programming, gaming, social media, online communities
- **Final Score:** `(base_score × user_weight) ÷ 3`

#### 3. **Collaboration Interests** (Activities to Do Together)

- **Exact Keywords:** D&D, teaching, learning, collaborative (+15 points each)
- **Complementary Matching** (+10 points):
  - "teach" ↔ "learn", "student", "beginner"
  - "mentor" ↔ "mentee", "guidance"  
  - "code" ↔ "programming", "development"
- **Category Matching** (+5 points): creative, technical, social, educational, gaming
- **Final Score:** `(base_score × user_weight) ÷ 15`

#### 4. **Food Preferences**

- **Exact Match:** 10 points
- **Cuisine Group Matching:** 6 points for same cuisine family
  - **Asian:** Chinese, Japanese, Thai, Korean, Vietnamese, Indian
  - **European:** Italian, French, German, Spanish, Greek
  - **Healthy:** Vegan, vegetarian, organic, salad
- **Final Score:** `(base_score × user_weight) ÷ 10`

#### 5. **Music Preferences**

- **Exact Match:** 10 points
- **Genre Group Matching:** 6 points for same genre family
  - **Rock:** rock, metal, punk, alternative, grunge
  - **Electronic:** techno, house, EDM, ambient, synth
  - **Jazz:** jazz, blues, swing, bebop
- **Final Score:** `(base_score × user_weight) ÷ 10`

#### 6. **Geographic Proximity** **Enhanced**

- **Radius Enforcement:** Hard cutoff at `max_radius_km` (SQL-level filtering)
- **Proximity Scoring:** Exponential decay curve `(1 - distance/radius)²`
- **Distance Bonuses:**
  - **≤2km:** +30% bonus (very local)
  - **≤5km:** +20% bonus (neighborhood)  
  - **≤10km:** +10% bonus (city level)
- **Special Cases:**
  - Weight = 0: No location scoring
  - Radius = 0: Moderate score for all distances

### User Preference Weighting

Each user sets **importance weights (0-10)** for each dimension:

- **0:** Completely ignore this factor
- **1-3:** Low importance  
- **4-6:** Moderate importance
- **7-10:** High importance

**Example:** If user sets `analog_passions: 8`, analog passion matches get 8× weight multiplier.

### Quality Assurance & Thresholds

#### **25% Minimum Compatibility Threshold**

- **Purpose:** Filter out low-quality matches
- **Calculation:** `(total_score ÷ sum_of_all_user_weights) × 100 ≥ 25%`
- **Implementation:** Applied during candidate scoring before ranking

#### **Result Management**

- **Maximum:** 10 recommendations per request
- **Sorting:** Highest compatibility scores first
- **State Management:** Tracks dismissed recommendations permanently

### Advanced Features

#### **Dismissal System**

- Users can permanently dismiss recommendations
- Dismissed users never reappear for that user
- Stored in `dismissed_recommendations` table

#### **Connection Integration**  

- Connected users (any status) excluded from recommendations
- Prevents recommending existing connections

#### **API Response Formats**

```json
// GET /recommendations
{"recommendations": [12, 46, 92, 10]}

// GET /recommendations/detailed  
{
  "recommendations": [
    {
      "user_id": 12,
      "score": 7,
      "score_percentage": 41.18,
      "distance": 3.81
    }
  ]
}
```

### Real-World Example

**User A Profile:**

- Analog: ["knitting", "art"] (weight: 8)
- Digital: ["programming"] (weight: 6)  
- Food: "Italian" (weight: 4)
- Location: Helsinki, 20km radius (weight: 7)

**Candidate B:**

- Analog: ["knitting", "crafts"]
- Digital: ["coding", "tech"]
- Food: "Italian"  
- Location: 5km from User A

**Score Calculation:**

- Analog: (3+1) × 8 ÷ 3 = **11 points**
- Digital: (1) × 6 ÷ 3 = **2 points**
- Food: (10) × 4 ÷ 10 = **4 points**  
- Location: (proximity + bonus) × 7 = **~21 points**
- **Total: ~38 points** → **~38% compatibility**

### Latest Enhancements (October 2025)

#### **Geographic Improvements**

- **Proper Radius Enforcement:** Hard filtering at SQL query level
- **Enhanced Proximity Scoring:** Exponential decay + distance bonuses  
- **Performance Optimization:** Early filtering reduces unnecessary calculations

#### **Scoring Refinements**  

- **Semantic Matching:** Better categorization of interests
- **Complementary Matching:** Enhanced "teach" + "learn" compatibility
- **Quality Thresholds:** 25% minimum score enforcement

#### **Frontend Integration**

- **Distance Display:** Shows km/m in profile modals
- **Score Percentages:** Compatibility badges on recommendation cards
- **Real-time Updates:** WebSocket integration for live status

**This is a transfer from the kood/sisu school gitea**
