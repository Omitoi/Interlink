# 🐳 Match-Me Docker Quick Reference

## Essential Commands

### Start/Stop Application

```bash
# Production
make start          # Start all services
make stop           # Stop all services

# Development  
make start-dev      # Start with hot reload
make stop-dev       # Stop development
```

### Service Management

```bash
docker compose ps           # Check service status
docker compose logs -f      # Follow logs
docker compose down         # Stop and remove containers
docker compose up -d        # Start in background
```

### Access Points

- **Frontend**: <http://localhost:3001>
- **Backend API**: <http://localhost:8081>
- **Database**: localhost:5433 (user: matchme_user)

### Useful Commands

```bash
make db-shell              # Access database shell
make backend-shell         # Access backend container
make clean                 # Remove all containers/images
./test-docker.sh          # Run health tests
```

### Development

```bash
# Start development with hot reload
make start-dev

# View development logs
```bash
docker compose -f docker-compose.dev.yml logs -f
```
