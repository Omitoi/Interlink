# Match-Me Docker Setup 🐳

Simple Docker setup for the Match-Me dating application.

## Prerequisites

- Docker & Docker Compose
- Make (optional, for convenience)

## Quick Start

### Production Mode

```bash
make up          # Start all services
make seed        # Populate with test data (100 users)
```

- Frontend: <http://localhost:3001>
- Backend API: <http://localhost:8081>

### Development Mode (with hot reload)

```bash
make dev         # Start with hot reload
make seed-dev    # Populate dev database
```

## Basic Commands

```bash
# Start/Stop
make up          # Start production
make dev         # Start development (hot reload)
make down        # Stop all services

# Database
make seed        # Add 100 test users (default)
make seed COUNT=50    # Add 50 test users
make seed-dev    # Add test users to dev database
make seed-dev COUNT=200  # Add 200 test users to dev
make clean-db    # Reset database (removes all data)

# Utilities
make logs        # View logs
make clean       # Remove containers and reset
```

## Test Data

After running `make seed`, you can login with:

- Email: `user1@test.local` to `user2@test.local`  
- Password: `test1234` (same for all test users)

The seeder creates users with varied interests, locations, and connection states for testing the matching algorithm.

### Development Tools

- Direct access to containers for debugging
- Separate development database
- Live log following
- Easy test execution

## 📝 File Structure

```go
match-me/
├── docker-compose.yml          # Production setup
├── docker-compose.dev.yml      # Development setup
├── Makefile                    # Convenience commands
├── .env                        # Production environment
├── .env.dev                    # Development environment
├── .dockerignore               # Docker ignore rules
├── backend/
│   ├── Dockerfile              # Production backend image
│   ├── Dockerfile.dev          # Development backend image
│   ├── .air.toml               # Hot reload configuration
│   └── .dockerignore
├── frontend/
│   ├── Dockerfile              # Production frontend image
│   ├── Dockerfile.dev          # Development frontend image
│   ├── nginx.conf              # Nginx configuration
│   └── .dockerignore
└── db/
    └── schema.sql              # Database schema
```

## 🚦 Troubleshooting

### Common Issues

1. **Port conflicts:**

   ```bash
   # Check what's using the port
   lsof -i :3000
   # Kill the process or change the port in .env
   ```

2. **Permission issues:**

   ```bash
   # Fix Docker permissions
   sudo chmod 666 /var/run/docker.sock
   ```

3. **Database connection issues:**

   ```bash
   # Check database logs
   make logs
   # Reset database
   make reset
   ```

4. **Build cache issues:**

   ```bash
   # Clean build cache
   docker builder prune -a
   make clean
   ```

### Health Checks

Check if services are healthy:

```bash
make status           # Production
make status-dev       # Development
```bash
docker compose ps     # Detailed status
```

### Logs

View service logs:

```bash
# All services
make logs-f

# Specific service
```bash
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
```

## 🔄 Updates and Maintenance

### Updating Images

```bash
# Pull latest base images
```bash
docker compose pull

# Rebuild with latest dependencies
make build
```

### Database Migrations

```bash
# Connect to database
make db-shell

# Run SQL commands
\c matchme_db
-- Your migration SQL here
```

### Backup and Restore

```bash
docker compose exec postgres pg_dump -U matchme_user matchme_db > backup.sql

# Restore
docker compose exec -T postgres psql -U matchme_user matchme_db < backup.sql
```

## 🎯 Production Deployment

For production deployment:

1. **Update environment variables** in `.env`
2. **Set secure passwords** and JWT secrets
3. **Configure domain/SSL** in nginx.conf
4. **Set up proper logging** and monitoring
5. **Configure backup strategy**

### Docker Compose Override

Create `docker-compose.override.yml` for production-specific settings:

```yaml
version: '3.8'
services:
  frontend:
    environment:
      - VIRTUAL_HOST=yourdomain.com
      - LETSENCRYPT_HOST=yourdomain.com
```

## 📞 Support

For issues with the Docker setup:

1. Check the troubleshooting section above
2. Review logs with `make logs-f`
3. Ensure all prerequisites are installed
4. Try cleaning and rebuilding with `make clean && make build`
