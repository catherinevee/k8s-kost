#!/bin/bash

# Kubernetes Cost Optimizer - Deployment Script
# This script automates the deployment of the K8s Cost Optimizer application

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
APP_NAME="k8s-cost-optimizer"
NAMESPACE="kube-system"
REGISTRY=${REGISTRY:-"your-registry.com"}
VERSION=${VERSION:-"v1.0.0"}

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed. Please install kubectl first."
    fi
    
    # Check if kubectl can connect to cluster
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
    fi
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker first."
    fi
    
    # Check if Make is installed
    if ! command -v make &> /dev/null; then
        error "Make is not installed. Please install Make first."
    fi
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go 1.19+ first."
    fi
    
    # Check if Node.js is installed
    if ! command -v node &> /dev/null; then
        error "Node.js is not installed. Please install Node.js 16+ first."
    fi
    
    success "All prerequisites are satisfied"
}

# Check if namespace exists
check_namespace() {
    log "Checking namespace..."
    
    if ! kubectl get namespace $NAMESPACE &> /dev/null; then
        log "Creating namespace $NAMESPACE..."
        kubectl create namespace $NAMESPACE
        success "Namespace $NAMESPACE created"
    else
        success "Namespace $NAMESPACE already exists"
    fi
}

# Setup database
setup_database() {
    log "Setting up database..."
    
    # Check if PostgreSQL is running
    if ! kubectl get deployment postgresql -n $NAMESPACE &> /dev/null; then
        log "Deploying PostgreSQL..."
        kubectl apply -f deploy/postgresql.yaml -n $NAMESPACE
        
        # Wait for PostgreSQL to be ready
        log "Waiting for PostgreSQL to be ready..."
        kubectl wait --for=condition=available --timeout=300s deployment/postgresql -n $NAMESPACE
        success "PostgreSQL is ready"
    else
        success "PostgreSQL is already deployed"
    fi
}

# Setup Redis
setup_redis() {
    log "Setting up Redis..."
    
    # Check if Redis is running
    if ! kubectl get deployment redis -n $NAMESPACE &> /dev/null; then
        log "Deploying Redis..."
        kubectl apply -f deploy/redis.yaml -n $NAMESPACE
        
        # Wait for Redis to be ready
        log "Waiting for Redis to be ready..."
        kubectl wait --for=condition=available --timeout=300s deployment/redis -n $NAMESPACE
        success "Redis is ready"
    else
        success "Redis is already deployed"
    fi
}

# Create secrets
create_secrets() {
    log "Creating secrets..."
    
    # Check if secrets already exist
    if kubectl get secret k8s-cost-optimizer-secrets -n $NAMESPACE &> /dev/null; then
        warning "Secrets already exist. Skipping secret creation."
        return
    fi
    
    # Check if .env file exists
    if [ ! -f .env ]; then
        error ".env file not found. Please create .env file with required configuration."
    fi
    
    # Source environment variables
    source .env
    
    # Create secrets
    kubectl create secret generic k8s-cost-optimizer-secrets \
        --from-literal=database-url="${DATABASE_URL:-postgresql://k8s_cost_user:password@postgresql:5432/k8s_cost_optimizer}" \
        --from-literal=redis-url="${REDIS_URL:-redis://redis:6379}" \
        --from-literal=aws-access-key="${AWS_ACCESS_KEY_ID:-}" \
        --from-literal=aws-secret-key="${AWS_SECRET_ACCESS_KEY:-}" \
        --from-literal=azure-client-id="${AZURE_CLIENT_ID:-}" \
        --from-literal=azure-client-secret="${AZURE_CLIENT_SECRET:-}" \
        --from-literal=gcp-credentials="${GCP_CREDENTIALS:-}" \
        -n $NAMESPACE
    
    success "Secrets created"
}

# Build and push Docker images
build_images() {
    log "Building Docker images..."
    
    # Check if registry is configured
    if [ "$REGISTRY" = "your-registry.com" ]; then
        warning "Using default registry. Please set REGISTRY environment variable for production."
    fi
    
    # Build images
    make docker-build
    
    # Push images
    log "Pushing Docker images..."
    make docker-push
    
    success "Docker images built and pushed"
}

# Deploy application
deploy_application() {
    log "Deploying application..."
    
    # Deploy Kubernetes resources
    kubectl apply -f deploy/kubernetes/deployment.yaml -n $NAMESPACE
    kubectl apply -f deploy/kubernetes/service.yaml -n $NAMESPACE
    kubectl apply -f deploy/kubernetes/ingress.yaml -n $NAMESPACE
    
    # Wait for deployment to be ready
    log "Waiting for application to be ready..."
    kubectl wait --for=condition=available --timeout=600s deployment/$APP_NAME -n $NAMESPACE
    
    success "Application deployed successfully"
}

# Deploy monitoring
deploy_monitoring() {
    log "Deploying monitoring..."
    
    # Deploy Prometheus rules
    if [ -f deploy/kubernetes/monitoring.yaml ]; then
    kubectl apply -f deploy/kubernetes/monitoring.yaml -n $NAMESPACE
    fi
    
    # Deploy Grafana dashboard
    if [ -f monitoring/grafana-dashboard.yaml ]; then
        kubectl apply -f monitoring/grafana-dashboard.yaml -n $NAMESPACE
    fi
    
    success "Monitoring deployed"
}

# Setup database migrations
setup_migrations() {
    log "Setting up database migrations..."
    
    # Wait a bit for database to be fully ready
    sleep 10
    
    # Run migrations
    make setup-db || warning "Database migrations failed. Please run manually: make setup-db"
}

# Check application status
check_status() {
    log "Checking application status..."
    
    echo ""
    echo "=== Application Status ==="
    kubectl get pods -n $NAMESPACE -l app=$APP_NAME
    echo ""
    echo "=== Services ==="
    kubectl get svc -n $NAMESPACE -l app=$APP_NAME
    echo ""
    echo "=== Ingress ==="
    kubectl get ingress -n $NAMESPACE -l app=$APP_NAME
    echo ""
    
    # Check if pods are running
    if kubectl get pods -n $NAMESPACE -l app=$APP_NAME --no-headers | grep -q "Running"; then
        success "Application is running successfully"
    else
        warning "Some pods are not running. Check logs with: kubectl logs -f deployment/$APP_NAME -n $NAMESPACE"
    fi
}

# Show access information
show_access_info() {
    log "Application access information:"
    echo ""
    echo "=== Access URLs ==="
    echo "Frontend: http://localhost:3000 (after port-forward)"
    echo "API: http://localhost:8080 (after port-forward)"
    echo ""
    echo "=== Port Forward Commands ==="
    echo "kubectl port-forward svc/$APP_NAME 3000:80 -n $NAMESPACE"
    echo "kubectl port-forward svc/$APP_NAME 8080:8080 -n $NAMESPACE"
    echo ""
    echo "=== Useful Commands ==="
    echo "View logs: kubectl logs -f deployment/$APP_NAME -n $NAMESPACE"
    echo "Check status: kubectl get pods -n $NAMESPACE -l app=$APP_NAME"
    echo "Scale: kubectl scale deployment $APP_NAME --replicas=3 -n $NAMESPACE"
    echo ""
}

# Main deployment function
main() {
    echo "=========================================="
    echo "Kubernetes Cost Optimizer - Deployment"
    echo "=========================================="
    echo ""
    
    # Check prerequisites
    check_prerequisites
    
    # Check namespace
    check_namespace
    
    # Setup infrastructure
    setup_database
    setup_redis
    
    # Create secrets
    create_secrets
    
    # Build and push images
    build_images
    
    # Deploy application
    deploy_application
    
    # Deploy monitoring
    deploy_monitoring
    
    # Setup migrations
    setup_migrations
    
    # Check status
    check_status
    
    # Show access information
    show_access_info
    
    echo "=========================================="
    success "Deployment completed successfully!"
    echo "=========================================="
}

# Help function
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -n, --namespace NAMESPACE  Specify namespace (default: kube-system)"
    echo "  -r, --registry REGISTRY    Specify Docker registry"
    echo "  -v, --version VERSION      Specify version (default: v1.0.0)"
    echo ""
    echo "Environment variables:"
    echo "  REGISTRY       Docker registry URL"
    echo "  VERSION        Application version"
    echo "  NAMESPACE      Kubernetes namespace"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Deploy with defaults"
    echo "  $0 -n my-namespace                    # Deploy to custom namespace"
    echo "  $0 -r my-registry.com -v v1.1.0       # Deploy with custom registry and version"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Run main function
main "$@" 