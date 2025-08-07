# Local Development Setup

This is a simple guide to run the Kubernetes Cost Optimizer application locally.

## Prerequisites

- Docker Desktop
- Go 1.19+
- Node.js 16+
- Make

## Quick Start (5 minutes)

### Step 1: Clone and Setup
```bash
cd k8s-kost
make install-deps
```

### Step 2: Start Local Environment
```bash
# Start infrastructure (PostgreSQL, Redis, Prometheus, Grafana)
make dev-local
```

### Step 3: Generate Sample Data
```bash
# Generate mock data for testing
make sample-data
```

### Step 4: Access the Application
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **Grafana**: http://localhost:3001 (admin/admin)
- **Prometheus**: http://localhost:9090

## Manual Setup (Alternative)

### Option 1: Using Docker Compose
```bash
# Start all services
docker-compose -f docker-compose.local.yml up -d

# Run database migrations
make setup-db-local

# Start development servers
make dev
```

### Option 2: Individual Services
```bash
# Start PostgreSQL
docker run -d --name postgres-local \
  -e POSTGRES_DB=k8s_cost_optimizer \
  -e POSTGRES_USER=k8s_cost_user \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  timescale/timescaledb:latest-pg14

# Start Redis
docker run -d --name redis-local \
  -p 6379:6379 \
  redis:7-alpine

# Start backend
cd backend && go run cmd/server/main.go

# Start frontend (in another terminal)
cd frontend && npm run dev
```

## Development Commands

```bash
# Start everything
make dev-all

# Stop everything
make dev-stop

# Restart
make dev-restart

# Clean up
make dev-clean

# Check status
make status

# View logs
make logs

# Reset database
make db-reset

# Generate sample data
make sample-data
```

## Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
lsof -i :8080
lsof -i :3000

# Kill the process
kill -9 <PID>
```

**Database connection error:**
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Restart PostgreSQL
docker restart postgres-local
```

**Docker not running:**
```bash
# Start Docker Desktop
# Then run:
make dev-local
```

### Getting Help

```bash
# Run diagnostics
make diagnose

# Check all services
make status

# View detailed logs
make logs
```

## What You Get

After setup, you'll have:

- **Cost Dashboard**: Real-time cost visualization
- **Optimization Recommendations**: Resource rightsizing suggestions
- **Sample Data**: 30 days of mock cost and metrics data
- **Monitoring**: Prometheus and Grafana dashboards
- **API**: Full REST API for integration

## Next Steps

1. Explore the dashboard at http://localhost:3000
2. Check the API documentation at http://localhost:8080
3. View metrics in Grafana at http://localhost:3001
4. Modify code and see live updates
5. Add new features and test locally

For detailed development information, see [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md). 