# Local Development Guide

This guide explains how to run the Kubernetes Cost Optimizer application locally for development and testing.

## Prerequisites

### Required Software
- **Docker Desktop** (with Kubernetes enabled)
- **Go 1.19+**
- **Node.js 16+**
- **kubectl** (configured for local cluster)
- **Make** (for build automation)
- **Git**

### Optional Software
- **PostgreSQL** (if not using Docker)
- **Redis** (if not using Docker)
- **Minikube** or **Kind** (alternative to Docker Desktop Kubernetes)

## Quick Local Setup

### Option 1: All-in-One Local Development (Recommended)

```bash
# Clone the repository
git clone <your-repo-url>
cd k8s-kost

# Install dependencies
make install-deps

# Start local development environment
make dev-local
```

### Option 2: Manual Setup

```bash
# 1. Start local Kubernetes cluster
docker run -d --name k8s-local \
  --privileged \
  -p 6443:6443 \
  -p 8080:8080 \
  -p 3000:3000 \
  kindest/node:v1.25.0

# 2. Configure kubectl for local cluster
kubectl config set-cluster local --server=https://localhost:6443
kubectl config set-context local --cluster=local
kubectl config use-context local

# 3. Deploy infrastructure
make deploy-infrastructure

# 4. Start development servers
make dev
```

## Detailed Local Development Setup

### Step 1: Environment Configuration

Create a local development environment file:

```bash
# Copy the example environment file
cp env.example .env.local

# Edit for local development
nano .env.local
```

**Local development settings:**

```bash
# Database Configuration (local)
DATABASE_URL=postgresql://k8s_cost_user:password@localhost:5432/k8s_cost_optimizer
REDIS_URL=redis://localhost:6379

# Application Configuration
PROMETHEUS_URL=http://localhost:9090
CLUSTER_NAME=local-cluster
NAMESPACE=default

# Development settings
LOG_LEVEL=debug
API_PORT=8080
FRONTEND_PORT=3000

# Mock cloud provider for local development
MOCK_CLOUD_PROVIDER=true
```

### Step 2: Start Local Infrastructure

#### Using Docker Compose (Recommended)

```bash
# Start PostgreSQL, Redis, and Prometheus
docker-compose -f docker-compose.local.yml up -d

# Verify services are running
docker-compose -f docker-compose.local.yml ps
```

#### Using Individual Docker Containers

```bash
# Start PostgreSQL with TimescaleDB
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

# Start Prometheus (optional)
docker run -d --name prometheus-local \
  -p 9090:9090 \
  -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus:latest
```

### Step 3: Database Setup

```bash
# Wait for PostgreSQL to be ready
sleep 10

# Run database migrations
make setup-db-local

# Verify database setup
psql -h localhost -U k8s_cost_user -d k8s_cost_optimizer -c "\dt"
```

### Step 4: Start Development Servers

#### Backend Development

```bash
# Navigate to backend directory
cd backend

# Install Go dependencies
go mod download

# Run backend in development mode
go run cmd/server/main.go
```

**Backend will be available at:** `http://localhost:8080`

#### Frontend Development

```bash
# Navigate to frontend directory
cd frontend

# Install Node.js dependencies
npm install

# Start development server
npm run dev
```

**Frontend will be available at:** `http://localhost:3000`

### Step 5: Verify Local Setup

```bash
# Check backend health
curl http://localhost:8080/health

# Check frontend
curl http://localhost:3000

# Check database connection
curl http://localhost:8080/api/health/db

# Check Redis connection
curl http://localhost:8080/api/health/cache
```

## Development Workflow

### Running Tests

```bash
# Backend tests
cd backend
go test -v ./...

# Frontend tests
cd frontend
npm test

# Integration tests
make test-integration
```

### Code Quality

```bash
# Backend linting
cd backend
golangci-lint run

# Frontend linting
cd frontend
npm run lint

# Format code
make format
```

### Building for Local Testing

```bash
# Build backend
make build-backend

# Build frontend
make build-frontend

# Build Docker images
make docker-build-local
```

## Local Kubernetes Development

### Using Minikube

```bash
# Start Minikube
minikube start --driver=docker

# Enable addons
minikube addons enable metrics-server
minikube addons enable ingress

# Deploy to Minikube
make deploy-minikube
```

### Using Kind

```bash
# Create Kind cluster
kind create cluster --name k8s-cost-optimizer

# Deploy to Kind
make deploy-kind
```

### Using Docker Desktop Kubernetes

```bash
# Enable Kubernetes in Docker Desktop
# Then deploy
make deploy-local
```

## Mock Data for Local Development

### Generate Sample Data

```bash
# Generate mock Kubernetes metrics
make generate-mock-metrics

# Generate mock cost data
make generate-mock-costs

# Generate mock recommendations
make generate-mock-recommendations
```

### Sample Data Scripts

```bash
# Run sample data generation
cd scripts
./generate-sample-data.sh

# Or use the Makefile
make sample-data
```

## Debugging

### Backend Debugging

```bash
# Run with debug logging
LOG_LEVEL=debug go run cmd/server/main.go

# Run with specific configuration
CONFIG_FILE=config/local.yaml go run cmd/server/main.go

# Debug with Delve
dlv debug cmd/server/main.go
```

### Frontend Debugging

```bash
# Start with debug mode
npm run dev:debug

# Check for linting issues
npm run lint:fix

# Run type checking
npm run type-check
```

### Database Debugging

```bash
# Connect to PostgreSQL
psql -h localhost -U k8s_cost_user -d k8s_cost_optimizer

# Check tables
\dt

# Check recent data
SELECT * FROM namespace_metrics ORDER BY timestamp DESC LIMIT 10;
```

## Common Local Development Issues

### Port Conflicts

```bash
# Check what's using port 8080
lsof -i :8080

# Check what's using port 3000
lsof -i :3000

# Kill process using port
kill -9 <PID>
```

### Database Connection Issues

```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Check PostgreSQL logs
docker logs postgres-local

# Restart PostgreSQL
docker restart postgres-local
```

### Redis Connection Issues

```bash
# Check if Redis is running
docker ps | grep redis

# Test Redis connection
docker exec -it redis-local redis-cli ping

# Restart Redis
docker restart redis-local
```

### Kubernetes Connection Issues

```bash
# Check cluster status
kubectl cluster-info

# Check nodes
kubectl get nodes

# Check pods
kubectl get pods --all-namespaces
```

## Performance Optimization for Local Development

### Backend Performance

```bash
# Run with profiling
go run -cpuprofile=cpu.prof cmd/server/main.go

# Run with memory profiling
go run -memprofile=mem.prof cmd/server/main.go

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Frontend Performance

```bash
# Build for production analysis
npm run build:analyze

# Check bundle size
npm run build:size

# Run performance audit
npm run audit
```

## Local Development Scripts

### Quick Development Commands

```bash
# Start everything for development
make dev-all

# Stop all development services
make dev-stop

# Restart development environment
make dev-restart

# Clean development environment
make dev-clean

# Update dependencies
make dev-update-deps
```

### Database Management

```bash
# Reset database
make db-reset

# Seed with sample data
make db-seed

# Backup database
make db-backup

# Restore database
make db-restore
```

## Integration with IDE

### VS Code Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Backend",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/backend/cmd/server/main.go",
      "env": {
        "LOG_LEVEL": "debug",
        "DATABASE_URL": "postgresql://k8s_cost_user:password@localhost:5432/k8s_cost_optimizer"
      }
    },
    {
      "name": "Launch Frontend",
      "type": "node",
      "request": "launch",
      "program": "${workspaceFolder}/frontend/node_modules/.bin/vite",
      "args": ["--port", "3000"],
      "cwd": "${workspaceFolder}/frontend"
    }
  ]
}
```

### IntelliJ IDEA Configuration

1. **Backend Run Configuration:**
   - Main class: `cmd/server/main.go`
   - Working directory: `backend/`
   - Environment variables: Set from `.env.local`

2. **Frontend Run Configuration:**
   - Package.json script: `dev`
   - Working directory: `frontend/`

## Troubleshooting

### Common Error Solutions

**"Connection refused" errors:**
```bash
# Check if services are running
docker ps

# Restart services
make dev-restart
```

**"Module not found" errors:**
```bash
# Update Go modules
cd backend && go mod tidy

# Update Node modules
cd frontend && npm install
```

**"Permission denied" errors:**
```bash
# Fix Docker permissions
sudo chmod 666 /var/run/docker.sock

# Or add user to docker group
sudo usermod -aG docker $USER
```

### Getting Help

```bash
# Check application logs
make logs

# Check system resources
make status

# Run diagnostics
make diagnose
```

## Next Steps

After setting up local development:

1. **Explore the codebase** - Start with `backend/cmd/server/main.go`
2. **Run the tests** - Ensure everything works correctly
3. **Make changes** - Modify code and see live updates
4. **Add features** - Implement new functionality
5. **Submit PRs** - Contribute back to the project

For more detailed information, see the main [README.md](README.md) and [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md). 