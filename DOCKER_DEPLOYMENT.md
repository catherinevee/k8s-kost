# Docker Deployment Guide

This guide shows you how to run the Kubernetes Cost Optimizer application using Docker containers.

## Prerequisites

### Required Software
- **Docker Desktop** (with Docker Compose)
- **Docker Engine** (for Linux)
- **Make** (for automation)
- **Git**

### System Requirements
- **RAM**: 4GB+ (8GB recommended)
- **Storage**: 10GB+ free space
- **CPU**: 2+ cores

## Quick Start (5 minutes)

### Option 1: All-in-One Docker Compose

```bash
# Clone the repository
git clone <your-repo-url>
cd k8s-kost

# Start everything with Docker Compose
docker-compose -f docker-compose.local.yml up -d

# Access the application
# Frontend: http://localhost:3000
# Backend API: http://localhost:8080
# Grafana: http://localhost:3001 (admin/admin)
# Prometheus: http://localhost:9090
```

### Option 2: Using Makefile

```bash
# Clone and setup
git clone <your-repo-url>
cd k8s-kost

# Start development environment
make dev-local

# Generate sample data
make sample-data
```

## Docker Deployment Options

### Option 1: Docker Compose (Recommended)

#### Full Stack Deployment

```bash
# Start all services including application
docker-compose -f docker-compose.local.yml --profile full-stack up -d

# Check status
docker-compose -f docker-compose.local.yml ps

# View logs
docker-compose -f docker-compose.local.yml logs -f
```

#### Infrastructure Only

```bash
# Start only infrastructure (PostgreSQL, Redis, Prometheus, Grafana)
docker-compose -f docker-compose.local.yml up -d

# Then run application separately
make dev
```

### Option 2: Individual Docker Containers

#### Start Infrastructure

```bash
# PostgreSQL with TimescaleDB
docker run -d --name postgres-k8s-cost \
  -e POSTGRES_DB=k8s_cost_optimizer \
  -e POSTGRES_USER=k8s_cost_user \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  -v postgres_data:/var/lib/postgresql/data \
  timescale/timescaledb:latest-pg14

# Redis
docker run -d --name redis-k8s-cost \
  -p 6379:6379 \
  -v redis_data:/data \
  redis:7-alpine redis-server --appendonly yes

# Prometheus
docker run -d --name prometheus-k8s-cost \
  -p 9090:9090 \
  -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml \
  -v prometheus_data:/prometheus \
  prom/prometheus:latest

# Grafana
docker run -d --name grafana-k8s-cost \
  -p 3001:3000 \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  -v grafana_data:/var/lib/grafana \
  grafana/grafana:latest
```

#### Run Application

```bash
# Build application image
docker build -t k8s-cost-optimizer:latest .

# Run backend
docker run -d --name backend-k8s-cost \
  -p 8080:8080 \
  -e DATABASE_URL=postgresql://k8s_cost_user:password@host.docker.internal:5432/k8s_cost_optimizer \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  -e PROMETHEUS_URL=http://host.docker.internal:9090 \
  -e MOCK_CLOUD_PROVIDER=true \
  k8s-cost-optimizer:latest

# Run frontend (if separate container)
docker run -d --name frontend-k8s-cost \
  -p 3000:3000 \
  -e REACT_APP_API_URL=http://localhost:8080 \
  k8s-cost-optimizer-frontend:latest
```

### Option 3: Production Docker Compose

```bash
# Create production environment file
cp env.example .env.prod
# Edit .env.prod with production settings

# Start production stack
docker-compose -f docker-compose.prod.yml up -d
```

## Docker Compose Configurations

### Development Configuration

The `docker-compose.local.yml` includes:

- **PostgreSQL** with TimescaleDB for time-series data
- **Redis** for caching with persistence
- **Prometheus** for metrics collection
- **Grafana** for dashboards
- **Backend** and **Frontend** containers (optional)
- **Health checks** and **volume persistence**

### Production Configuration

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  postgres:
    image: timescale/timescaledb:latest-pg14
    environment:
      POSTGRES_DB: k8s_cost_optimizer
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER} -d k8s_cost_optimizer"]
      interval: 30s
      timeout: 10s
      retries: 3

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes --maxmemory 1gb --maxmemory-policy allkeys-lru
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3

  backend:
    image: ${REGISTRY}/k8s-cost-optimizer:${VERSION}
    environment:
      - DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@postgres:5432/k8s_cost_optimizer
      - REDIS_URL=redis://redis:6379
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=${AWS_REGION}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  frontend:
    image: ${REGISTRY}/k8s-cost-optimizer-frontend:${VERSION}
    environment:
      - REACT_APP_API_URL=http://backend:8080
    depends_on:
      - backend
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - frontend
      - backend
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
```

## Docker Image Building

### Build All Images

```bash
# Build backend image
docker build -t k8s-cost-optimizer:latest .

# Build frontend image
docker build -t k8s-cost-optimizer-frontend:latest -f docker/Dockerfile.frontend ./frontend

# Build with specific version
docker build -t k8s-cost-optimizer:v1.0.0 .
```

### Multi-stage Builds

The Dockerfile uses multi-stage builds for optimization:

```dockerfile
# Backend Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

### Frontend Dockerfile

```dockerfile
# Frontend Dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## Environment Configuration

### Development Environment

Create `.env.local`:

```bash
# Database Configuration
DATABASE_URL=postgresql://k8s_cost_user:password@postgres:5432/k8s_cost_optimizer
REDIS_URL=redis://redis:6379

# Application Configuration
PROMETHEUS_URL=http://prometheus:9090
CLUSTER_NAME=docker-cluster
NAMESPACE=default

# Development settings
LOG_LEVEL=debug
API_PORT=8080
FRONTEND_PORT=3000

# Mock cloud provider for development
MOCK_CLOUD_PROVIDER=true
```

### Production Environment

Create `.env.prod`:

```bash
# Database Configuration
DATABASE_URL=postgresql://k8s_cost_user:secure_password@postgres:5432/k8s_cost_optimizer
REDIS_URL=redis://redis:6379

# Cloud Provider Configuration
AWS_ACCESS_KEY_ID=your_production_aws_key
AWS_SECRET_ACCESS_KEY=your_production_aws_secret
AWS_REGION=us-west-2

# Application Configuration
PROMETHEUS_URL=http://prometheus:9090
CLUSTER_NAME=production-cluster
NAMESPACE=default

# Security
JWT_SECRET=your_secure_jwt_secret
ENCRYPTION_KEY=your_secure_encryption_key

# Performance
CACHE_TTL=900
METRICS_RETENTION_DAYS=90
MAX_CONCURRENT_REQUESTS=100
```

## Database Setup

### Initialize Database

```bash
# Wait for PostgreSQL to be ready
docker exec postgres-k8s-cost pg_isready -U k8s_cost_user -d k8s_cost_optimizer

# Run migrations
docker exec -i postgres-k8s-cost psql -U k8s_cost_user -d k8s_cost_optimizer < backend/internal/database/migrations.sql

# Or use the Makefile
make setup-db-local
```

### Generate Sample Data

```bash
# Generate mock data for testing
make sample-data

# Or run manually
docker exec -it postgres-k8s-cost psql -U k8s_cost_user -d k8s_cost_optimizer -c "
INSERT INTO namespace_metrics (namespace, metric_type, value, timestamp)
VALUES ('default', 'cpu_millicores', 250, NOW());
"
```

## Access Methods

### Port Forwarding

```bash
# Frontend
http://localhost:3000

# Backend API
http://localhost:8080

# Grafana
http://localhost:3001 (admin/admin)

# Prometheus
http://localhost:9090
```

### Container Networking

```bash
# Check container network
docker network ls
docker network inspect k8s-kost_k8s-cost-network

# Connect to container
docker exec -it backend-k8s-cost sh
docker exec -it postgres-k8s-cost psql -U k8s_cost_user -d k8s_cost_optimizer
```

## Monitoring and Logs

### View Logs

```bash
# All services
docker-compose -f docker-compose.local.yml logs -f

# Specific service
docker-compose -f docker-compose.local.yml logs -f backend

# Individual containers
docker logs -f backend-k8s-cost
docker logs -f postgres-k8s-cost
```

### Health Checks

```bash
# Check container health
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Test application health
curl http://localhost:8080/health

# Test database connection
docker exec postgres-k8s-cost pg_isready -U k8s_cost_user -d k8s_cost_optimizer

# Test Redis connection
docker exec redis-k8s-cost redis-cli ping
```

### Resource Usage

```bash
# Check resource usage
docker stats

# Check disk usage
docker system df

# Check volume usage
docker volume ls
docker volume inspect k8s-kost_postgres_data
```

## Scaling and Performance

### Horizontal Scaling

```bash
# Scale backend containers
docker-compose -f docker-compose.local.yml up -d --scale backend=3

# Scale with load balancer
docker run -d --name nginx-lb \
  -p 80:80 \
  -v $(pwd)/nginx-lb.conf:/etc/nginx/nginx.conf \
  nginx:alpine
```

### Performance Optimization

```bash
# Optimize PostgreSQL
docker exec postgres-k8s-cost psql -U k8s_cost_user -d k8s_cost_optimizer -c "
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
SELECT pg_reload_conf();
"

# Optimize Redis
docker exec redis-k8s-cost redis-cli CONFIG SET maxmemory 1gb
docker exec redis-k8s-cost redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

## Backup and Recovery

### Database Backup

```bash
# Create backup
docker exec postgres-k8s-cost pg_dump -U k8s_cost_user k8s_cost_optimizer > backup.sql

# Restore backup
docker exec -i postgres-k8s-cost psql -U k8s_cost_user -d k8s_cost_optimizer < backup.sql

# Automated backup script
docker run --rm \
  -v $(pwd)/backups:/backups \
  --network k8s-kost_k8s-cost-network \
  postgres:14 \
  pg_dump -h postgres -U k8s_cost_user k8s_cost_optimizer > backups/backup-$(date +%Y%m%d).sql
```

### Volume Backup

```bash
# Backup volumes
docker run --rm -v k8s-kost_postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/postgres-backup.tar.gz -C /data .

# Restore volumes
docker run --rm -v k8s-kost_postgres_data:/data -v $(pwd):/backup alpine tar xzf /backup/postgres-backup.tar.gz -C /data
```

## Security

### Container Security

```bash
# Run containers with non-root user
docker run --user 1000:1000 k8s-cost-optimizer:latest

# Limit container resources
docker run --memory=512m --cpus=1 k8s-cost-optimizer:latest

# Use secrets for sensitive data
docker secret create db_password ./db_password.txt
docker run --secret db_password postgres:14
```

### Network Security

```bash
# Create custom network
docker network create --driver bridge --subnet=172.20.0.0/16 k8s-cost-network

# Run containers on custom network
docker run --network k8s-cost-network k8s-cost-optimizer:latest

# Use host networking (not recommended for production)
docker run --network host k8s-cost-optimizer:latest
```

## Troubleshooting

### Common Issues

**Container won't start:**
```bash
# Check container logs
docker logs container-name

# Check container configuration
docker inspect container-name

# Check resource limits
docker stats container-name
```

**Database connection issues:**
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Check PostgreSQL logs
docker logs postgres-k8s-cost

# Test connection from host
docker exec postgres-k8s-cost pg_isready -U k8s_cost_user -d k8s_cost_optimizer
```

**Port conflicts:**
```bash
# Check what's using the port
lsof -i :8080
lsof -i :3000

# Kill process using port
kill -9 <PID>

# Use different ports
docker run -p 8081:8080 k8s-cost-optimizer:latest
```

**Volume issues:**
```bash
# Check volume permissions
docker run --rm -v k8s-kost_postgres_data:/data alpine ls -la /data

# Fix volume permissions
docker run --rm -v k8s-kost_postgres_data:/data alpine chown -R 999:999 /data
```

### Debug Commands

```bash
# Get detailed status
docker-compose -f docker-compose.local.yml ps

# Check all containers
docker ps -a

# Check resource usage
docker stats --no-stream

# Check disk usage
docker system df

# Clean up unused resources
docker system prune -a
```

## Production Deployment

### Production Checklist

- [ ] **Security**: Non-root users, resource limits, secrets management
- [ ] **Monitoring**: Health checks, logging, metrics collection
- [ ] **Backup**: Automated database and volume backups
- [ ] **Scaling**: Load balancing, horizontal scaling
- [ ] **SSL/TLS**: HTTPS termination with reverse proxy
- [ ] **Updates**: Rolling updates, zero-downtime deployments
- [ ] **Documentation**: Runbooks, troubleshooting guides

### Production Docker Compose

```bash
# Use production configuration
docker-compose -f docker-compose.prod.yml up -d

# Use external secrets
docker secret create db_password ./db_password.txt
docker secret create aws_key ./aws_key.txt

# Use external volumes
docker volume create postgres_prod_data
docker volume create redis_prod_data
```

## Next Steps

After setting up Docker deployment:

1. **Configure monitoring** - Set up Prometheus and Grafana dashboards
2. **Set up CI/CD** - Automate image building and deployment
3. **Configure backup** - Implement automated backup strategy
4. **Performance tuning** - Optimize based on usage patterns
5. **Security hardening** - Implement additional security measures

For more detailed information, see the main [README.md](README.md) and [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md). 