# Match-Me Simple Makefile

.PHONY: help up dev down seed seed-dev clean-db logs clean status health build
.DEFAULT_GOAL := help

# Default help
help:
	@echo "Match-Me Commands:"
	@echo "=================="
	@echo "  easy      - Start everything up and seed database with 100 users"
	@echo "  up        - Start production environment"
	@echo "  dev       - Start development environment (hot reload)"
	@echo "  down      - Stop all services"
	@echo "  seed      - Populate database with test users (default: 100, truncates first)"
	@echo "              Usage: make seed COUNT=50"
	@echo "  seed-dev  - Populate dev database with test users (truncates first)"
	@echo "              Usage: make seed-dev COUNT=50"
	@echo "  clean-db  - Reset database (removes all data)"
	@echo "  logs      - View logs"
	@echo "  clean     - Remove containers and reset everything"
	@echo "  status    - Show status of all services"
	@echo "  health    - Check health of running services"
	@echo "  build     - Build all images without starting"

easy:
	@echo "Starting everything up and seeding database with 100 users..."
	make up && sleep 10 && make seed

# Build images without starting
build:
	@echo "🔨 Building all images..."
	docker compose build

# Start production
up:
	@echo "🚀 Starting Match-Me (production)..."
	docker compose up -d
	@echo "✅ Frontend running at http://localhost:3001"
	@echo "✅ Backend API running at http://localhost:8081"
	@echo "✅ Run 'make health' to verify all services are running"

# Start development with hot reload
dev:
	@echo "🚀 Starting Match-Me (development with hot reload)..."
	docker compose -f docker-compose.dev.yml up -d
	@echo "✅ Frontend (dev server) at http://localhost:3001"
	@echo "✅ Backend API (dev) at http://localhost:8081"
	@echo "✅ Run 'make health' to verify all services are running"

# Stop everything
down:
	@echo "🛑 Stopping services..."
	docker compose down
	docker compose -f docker-compose.dev.yml down

# Show service status
status:
	@echo "📊 Service Status:"
	@echo "=================="
	@docker compose ps --format "table {{.Service}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "Production services not running"
	@echo ""
	@docker compose -f docker-compose.dev.yml ps --format "table {{.Service}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "Development services not running"

# Check service health
health:
	@echo "🏥 Health Check:"
	@echo "================"
	@./test-docker.sh

# Seed production database
seed:
	@echo "🌱 Seeding database with $(or $(COUNT),100) test users (truncating existing data)..."
	@echo "Waiting for database to be ready..."
	@sleep 10
	@if ! docker compose ps postgres --format "{{.Status}}" | grep -q "Up"; then \
		echo "❌ Database not running. Run 'make up' first."; \
		exit 1; \
	fi
	cd db-seeder && DATABASE_URL="postgres://matchme_user:matchme_password@localhost:5433/matchme_db?sslmode=disable" go run main.go -count=$(or $(COUNT),100) -truncate

# Seed development database  
seed-dev:
	@echo "🌱 Seeding development database with $(or $(COUNT),100) test users (truncating existing data)..."
	@echo "Waiting for database to be ready..."
	@sleep 10
	@if ! docker compose -f docker-compose.dev.yml ps postgres --format "{{.Status}}" | grep -q "Up"; then \
		echo "❌ Development database not running. Run 'make dev' first."; \
		exit 1; \
	fi
	cd db-seeder && DATABASE_URL="postgres://matchme_user:matchme_password@localhost:5433/matchme_db?sslmode=disable" go run main.go -count=$(or $(COUNT),100) -truncate

# Reset database
clean-db:
	@echo "🗑️  Resetting database..."
	docker compose down -v
	docker compose -f docker-compose.dev.yml down -v

# View logs
logs:
	@echo "📋 Showing logs..."
	docker compose logs -f

# Clean everything
clean:
	@echo "🧹 Cleaning up everything..."
	docker compose down -v --rmi all
	docker compose -f docker-compose.dev.yml down -v --rmi all
	docker system prune -f
