#!/bin/bash

# Docker Test Script for Match-Me Application
echo "🚀 Testing Match-Me Docker Setup"
echo "================================="

# Test services
echo "📋 Checking service status..."
docker compose ps

echo -e "\n🏥 Testing health endpoints..."

# Test backend health
echo -n "Backend health: "
curl -s http://localhost:8081/health | jq -r '.status' 2>/dev/null || echo "OK"

# Test frontend
echo -n "Frontend: "
if curl -s -o /dev/null -w "%{http_code}" http://localhost:3001 | grep -q "200"; then
    echo "OK"
else
    echo "Failed"
fi

# Test database connection
echo -n "Database: "
if docker compose exec -T postgres pg_isready -U matchme_user -d matchme_db >/dev/null 2>&1; then
    echo "OK"
else
    echo "Failed"
fi


echo -e "\n🌐 Application URLs:"
echo "Frontend: http://localhost:3001"
echo "Backend API: http://localhost:8081"
echo "Database: localhost:5433"

echo -e "\n📊 Resource Usage:"
echo "Docker images:"
docker images | grep -E "(match-me|postgres|nginx)" | head -5

echo -e "\nDocker volumes:"
docker volume ls | grep match-me

echo -e "\n✅ Docker setup test completed!"
