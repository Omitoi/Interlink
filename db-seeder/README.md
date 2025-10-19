# Match Me Database Seeder

This is a standalone Go CLI tool for generating **realistic test data** into the Match Me PostgreSQL database.  
It creates:

- **Users** with hashed passwords
- **Profiles** with names, bios, JSON hobby fields, and locations
- **Connections** (accepted, pending, disconnected)
- **Dismissed recommendations** (used to filter recommendations)
- **Two Users to Test Chat** `user1@test.local` and `user2@test.local` connected to test chat functionality conveniently both with the default password (test1234)

The seeder is deterministic: with the same random seed and count, you’ll always get the same dataset.

---

## Requirements

- Go 1.21+
- Access to a PostgreSQL instance with the Match Me schema already migrated
- Environment variable or flag with a valid **DSN** (Postgres connection string)

---

## Installation

Clone the repo and build:

```bash
cd match-me/db-seeder
go build -o seed ./cmd/seed
```

This will produce a binary called `seed`.

---

## Usage

```bash
./seed --dsn "postgres://user:pass@localhost:5433/matchme?sslmode=disable" [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dsn` | `$DATABASE_URL` | Postgres DSN (e.g. `postgres://user:pass@localhost:5433/matchme?sslmode=disable`) |
| `--count` | `300` | Number of users to generate |
| `--seed` | `42` | Random seed for deterministic datasets |
| `--truncate` | `false` | If true, truncates `users`, `profiles`, `connections`, and `dismissed_recommendations` before inserting |
| `--connect-rate` | `0.60` | Fraction of users with **accepted connections** |
| `--pending-rate` | `0.10` | Fraction of users with **pending requests** |
| `--disconnected-rate` | `0.05` | Fraction of users with **disconnected past connections** |
| `--dismiss-rate` | `0.20` | Fraction of users with dismissed recommendations |
| `--password` | `"test1234"` | Password assigned to all generated users (hashed with bcrypt) |

---

## Example

Seed a local database with 500 users, truncating existing data:

```bash
export DATABASE_URL="postgres://matchme_user:matchme_password@localhost:5433/matchme_db?sslmode=disable"

./seed   --dsn "$DATABASE_URL"   --count 500   --truncate   --seed 1337   --connect-rate 0.6   --pending-rate 0.1   --disconnected-rate 0.05   --dismiss-rate 0.2
```

Now you can log in as any generated user:

- **Email**: e.g. `alex.korhonen+123456@example.com`  
- **Password**: `test1234` (unless overridden with `--password`)

---

## Notes

- The tool runs in a single transaction: if anything fails, no data is committed.
- Emails are guaranteed unique and look realistic (`firstname.lastname+random@example.com`).
- Locations are spread across Finnish cities with slight random latitude/longitude offsets.
- JSON fields (`analog_passions`, `digital_delights`, `other_bio`, `match_preferences`) are filled with randomized but valid values.
- The dismissed recommendations table is populated so users won’t see the same connections reappear in recommendations.

---

## Development

Run directly with Go:

```bash
go run ./cmd/seed --dsn "$DATABASE_URL" --count 100
```
