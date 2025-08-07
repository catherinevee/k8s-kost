# Kubernetes Cost Optimizer Makefile

# Variables
APP_NAME = k8s-cost-optimizer
VERSION ?= v1.0.0
REGISTRY ?= your-registry
NAMESPACE ?= kube-system
KUBECONFIG ?= ~/.kube/config

# Docker images
BACKEND_IMAGE = $(REGISTRY)/$(APP_NAME)-backend:$(VERSION)
FRONTEND_IMAGE = $(REGISTRY)/$(APP_NAME)-frontend:$(VERSION)

# Directories
BACKEND_DIR = backend
FRONTEND_DIR = frontend
DEPLOY_DIR = deploy/kubernetes

.PHONY: help build test deploy clean docker-build docker-push

# Default target
help:
	@echo "Kubernetes Cost Optimizer - Available targets:"
	@echo "  build          - Build backend and frontend"
	@echo "  test           - Run tests"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-push    - Push Docker images to registry"
	@echo "  deploy         - Deploy to Kubernetes"
	@echo "  clean          - Clean build artifacts"
	@echo "  setup-db       - Setup database and run migrations"
	@echo "  lint           - Run linting"
	@echo "  security-scan  - Run security scans"

# Build backend and frontend
build: build-backend build-frontend

build-backend:
	@echo "Building backend..."
	cd $(BACKEND_DIR) && go build -o bin/server ./cmd/server

build-frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm install && npm run build

# Run tests
test: test-backend test-frontend

test-backend:
	@echo "Running backend tests..."
	cd $(BACKEND_DIR) && go test -v ./...

test-frontend:
	@echo "Running frontend tests..."
	cd $(FRONTEND_DIR) && npm test

# Docker operations
docker-build:
	@echo "Building Docker images..."
	docker build -t $(BACKEND_IMAGE) -f docker/Dockerfile .
	docker build -t $(FRONTEND_IMAGE) -f docker/Dockerfile.frontend .

docker-push:
	@echo "Pushing Docker images..."
	docker push $(BACKEND_IMAGE)
	docker push $(FRONTEND_IMAGE)

# Kubernetes deployment
deploy: deploy-namespace deploy-secrets deploy-config deploy-app deploy-monitoring

deploy-namespace:
	@echo "Creating namespace..."
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

deploy-secrets:
	@echo "Deploying secrets..."
	kubectl apply -f $(DEPLOY_DIR)/secrets.yaml -n $(NAMESPACE)

deploy-config:
	@echo "Deploying configuration..."
	kubectl apply -f $(DEPLOY_DIR)/configmap.yaml -n $(NAMESPACE)

deploy-app:
	@echo "Deploying application..."
	kubectl apply -f $(DEPLOY_DIR)/deployment.yaml -n $(NAMESPACE)

deploy-monitoring:
	@echo "Deploying monitoring..."
	kubectl apply -f $(DEPLOY_DIR)/enhanced-monitoring.yaml -n $(NAMESPACE)

# Database setup
setup-db:
	@echo "Setting up database..."
	@echo "Please ensure PostgreSQL is running and accessible"
	@echo "Running database migrations..."
	cd $(BACKEND_DIR) && go run cmd/migrate/main.go

# Linting
lint: lint-backend lint-frontend

lint-backend:
	@echo "Linting backend..."
	cd $(BACKEND_DIR) && golangci-lint run

lint-frontend:
	@echo "Linting frontend..."
	cd $(FRONTEND_DIR) && npm run lint

# Security scanning
security-scan:
	@echo "Running security scans..."
	@echo "Backend security scan..."
	cd $(BACKEND_DIR) && gosec ./...
	@echo "Frontend security scan..."
	cd $(FRONTEND_DIR) && npm audit

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BACKEND_DIR)/bin
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules
	docker system prune -f

# Development helpers
dev-backend:
	@echo "Starting backend in development mode..."
	cd $(BACKEND_DIR) && go run cmd/server/main.go

dev-frontend:
	@echo "Starting frontend in development mode..."
	cd $(FRONTEND_DIR) && npm run dev

dev: dev-backend dev-frontend

# Local development with Docker Compose
dev-local:
	@echo "Starting local development environment..."
	docker-compose -f docker-compose.local.yml up -d postgres redis prometheus grafana
	@echo "Waiting for services to be ready..."
	sleep 15
	@echo "Running database migrations..."
	make setup-db-local
	@echo "Starting development servers..."
	make dev

dev-all:
	@echo "Starting full development stack..."
	docker-compose -f docker-compose.local.yml --profile full-stack up -d

dev-stop:
	@echo "Stopping development environment..."
	docker-compose -f docker-compose.local.yml down

dev-restart:
	@echo "Restarting development environment..."
	make dev-stop
	make dev-local

dev-clean:
	@echo "Cleaning development environment..."
	docker-compose -f docker-compose.local.yml down -v
	docker system prune -f

# Local database management
setup-db-local:
	@echo "Setting up local database..."
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker exec k8s-cost-postgres pg_isready -U k8s_cost_user -d k8s_cost_optimizer; do sleep 2; done
	@echo "Running migrations..."
	docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -f /docker-entrypoint-initdb.d/01-migrations.sql

db-reset:
	@echo "Resetting local database..."
	docker-compose -f docker-compose.local.yml down postgres
	docker volume rm k8s-kost_postgres_data || true
	docker-compose -f docker-compose.local.yml up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	sleep 15
	make setup-db-local

db-seed:
	@echo "Seeding database with sample data..."
	cd scripts && ./generate-sample-data.sh

# Mock data generation
generate-mock-metrics:
	@echo "Generating mock Kubernetes metrics..."
	cd scripts && ./generate-mock-metrics.sh

generate-mock-costs:
	@echo "Generating mock cost data..."
	cd scripts && ./generate-mock-costs.sh

generate-mock-recommendations:
	@echo "Generating mock recommendations..."
	cd scripts && ./generate-mock-recommendations.sh

sample-data: generate-mock-metrics generate-mock-costs generate-mock-recommendations

# Local Kubernetes development
deploy-minikube:
	@echo "Deploying to Minikube..."
	minikube start --driver=docker
	minikube addons enable metrics-server
	minikube addons enable ingress
	make deploy

deploy-kind:
	@echo "Deploying to Kind..."
	kind create cluster --name k8s-cost-optimizer || true
	make deploy

deploy-local:
	@echo "Deploying to local Kubernetes..."
	make deploy

# Development utilities
logs:
	@echo "Showing application logs..."
	docker-compose -f docker-compose.local.yml logs -f

status:
	@echo "Development environment status:"
	docker-compose -f docker-compose.local.yml ps
	@echo ""
	@echo "Backend health:"
	@curl -s http://localhost:8080/health || echo "Backend not running"
	@echo ""
	@echo "Frontend:"
	@curl -s http://localhost:3000 > /dev/null && echo "Frontend running" || echo "Frontend not running"

diagnose:
	@echo "Running diagnostics..."
	@echo "Docker status:"
	docker info > /dev/null && echo "✓ Docker running" || echo "✗ Docker not running"
	@echo ""
	@echo "Kubernetes status:"
	kubectl cluster-info > /dev/null && echo "✓ Kubernetes accessible" || echo "✗ Kubernetes not accessible"
	@echo ""
	@echo "Port availability:"
	@lsof -i :8080 > /dev/null && echo "✗ Port 8080 in use" || echo "✓ Port 8080 available"
	@lsof -i :3000 > /dev/null && echo "✗ Port 3000 in use" || echo "✓ Port 3000 available"

# Port forwarding for local development
port-forward:
	@echo "Setting up port forwarding..."
	kubectl port-forward svc/$(APP_NAME) 3000:80 -n $(NAMESPACE) &
	kubectl port-forward svc/$(APP_NAME) 8080:8080 -n $(NAMESPACE) &

# Logs
logs:
	kubectl logs -f deployment/$(APP_NAME) -n $(NAMESPACE)

# Status
status:
	@echo "Application status:"
	kubectl get pods -n $(NAMESPACE) -l app=$(APP_NAME)
	kubectl get svc -n $(NAMESPACE) -l app=$(APP_NAME)

# Scale
scale:
	@echo "Scaling application..."
	kubectl scale deployment $(APP_NAME) --replicas=$(REPLICAS) -n $(NAMESPACE)

# Update
update:
	@echo "Updating application..."
	kubectl set image deployment/$(APP_NAME) backend=$(BACKEND_IMAGE) -n $(NAMESPACE)
	kubectl set image deployment/$(APP_NAME) frontend=$(FRONTEND_IMAGE) -n $(NAMESPACE)

# Rollback
rollback:
	@echo "Rolling back application..."
	kubectl rollout undo deployment/$(APP_NAME) -n $(NAMESPACE)

# Backup database
backup-db:
	@echo "Backing up database..."
	@echo "Please implement database backup logic"

# Restore database
restore-db:
	@echo "Restoring database..."
	@echo "Please implement database restore logic"

# Generate documentation
docs:
	@echo "Generating documentation..."
	@echo "Please implement documentation generation"

# Performance testing
perf-test:
	@echo "Running performance tests..."
	@echo "Please implement performance testing"

# Load testing
load-test:
	@echo "Running load tests..."
	@echo "Please implement load testing"

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	# Backend dependencies
	cd $(BACKEND_DIR) && go mod download
	# Frontend dependencies
	cd $(FRONTEND_DIR) && npm install
	# Development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Setup development environment
setup-dev: install-deps
	@echo "Setting up development environment..."
	@echo "Please ensure you have:"
	@echo "  - Go 1.21+"
	@echo "  - Node.js 18+"
	@echo "  - Docker"
	@echo "  - kubectl"
	@echo "  - A Kubernetes cluster"

# Production deployment
deploy-prod: docker-build docker-push
	@echo "Deploying to production..."
	$(MAKE) deploy NAMESPACE=production

# Staging deployment
deploy-staging: docker-build docker-push
	@echo "Deploying to staging..."
	$(MAKE) deploy NAMESPACE=staging

# Local deployment (minikube/kind)
deploy-local:
	@echo "Deploying to local cluster..."
	$(MAKE) deploy NAMESPACE=default

# Uninstall
uninstall:
	@echo "Uninstalling application..."
	kubectl delete -f $(DEPLOY_DIR)/enhanced-monitoring.yaml -n $(NAMESPACE) --ignore-not-found
	kubectl delete -f $(DEPLOY_DIR)/deployment.yaml -n $(NAMESPACE) --ignore-not-found
	kubectl delete -f $(DEPLOY_DIR)/configmap.yaml -n $(NAMESPACE) --ignore-not-found
	kubectl delete -f $(DEPLOY_DIR)/secrets.yaml -n $(NAMESPACE) --ignore-not-found

# Show help for specific target
help-%:
	@echo "Help for target '$*':"
	@grep -A 1 "^$*:" Makefile | tail -1 