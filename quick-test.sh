#!/bin/bash

# Kubernetes Cost Optimizer - Quick Test Script
# This script provides a fast way to test the application locally

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="k8s-cost-optimizer"
DOCKER_COMPOSE_FILE="docker-compose.local.yml"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}================================${NC}"
    echo -e "${PURPLE} $1${NC}"
    echo -e "${PURPLE}================================${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker Desktop."
        exit 1
    fi
    print_success "Docker is running"
    
    # Check if Docker Compose is available
    if ! command -v docker-compose >/dev/null 2>&1; then
        print_error "Docker Compose is not installed."
        exit 1
    fi
    print_success "Docker Compose is available"
    
    # Check if Make is available
    if ! command -v make >/dev/null 2>&1; then
        print_warning "Make is not available. Some commands may not work."
    else
        print_success "Make is available"
    fi
    
    # Check if ports are available
    local ports=(3000 3001 8080 9090 5432 6379)
    for port in "${ports[@]}"; do
        if lsof -i :$port >/dev/null 2>&1; then
            print_warning "Port $port is already in use"
        else
            print_success "Port $port is available"
        fi
    done
}

# Function to start the application
start_application() {
    print_header "Starting Kubernetes Cost Optimizer"
    
    # Navigate to project directory
    cd "$SCRIPT_DIR"
    
    # Create .env file if it doesn't exist
    if [ ! -f .env ]; then
        print_status "Creating .env file from template..."
        cp env.example .env
        print_success "Created .env file"
    fi
    
    # Start infrastructure services
    print_status "Starting infrastructure services..."
    docker-compose -f $DOCKER_COMPOSE_FILE up -d postgres redis prometheus grafana
    
    # Wait for services to be ready
    print_status "Waiting for services to be ready..."
    sleep 10
    
    # Setup database
    print_status "Setting up database..."
    if command -v make >/dev/null 2>&1; then
        make setup-db-local
    else
        # Manual database setup
        docker exec k8s-kost-postgres-1 pg_isready -U k8s_cost_user -d k8s_cost_optimizer || {
            print_error "Database is not ready. Retrying..."
            sleep 5
            docker exec k8s-kost-postgres-1 pg_isready -U k8s_cost_user -d k8s_cost_optimizer
        }
        
        # Run migrations
        docker exec -i k8s-kost-postgres-1 psql -U k8s_cost_user -d k8s_cost_optimizer < backend/internal/database/migrations.sql
    fi
    print_success "Database setup complete"
    
    # Generate sample data
    print_status "Generating sample data..."
    if command -v make >/dev/null 2>&1; then
        make sample-data
    else
        # Manual sample data generation
        ./scripts/generate-sample-data.sh
    fi
    print_success "Sample data generated"
    
    # Start application services
    print_status "Starting application services..."
    docker-compose -f $DOCKER_COMPOSE_FILE up -d backend frontend
    
    print_success "Application started successfully!"
}

# Function to check application status
check_status() {
    print_header "Application Status"
    
    cd "$SCRIPT_DIR"
    
    # Check if containers are running
    print_status "Checking container status..."
    docker-compose -f $DOCKER_COMPOSE_FILE ps
    
    # Check application health
    print_status "Checking application health..."
    
    # Check backend health
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        print_success "Backend is healthy"
    else
        print_error "Backend is not responding"
    fi
    
    # Check frontend
    if curl -f http://localhost:3000 >/dev/null 2>&1; then
        print_success "Frontend is accessible"
    else
        print_error "Frontend is not responding"
    fi
    
    # Check Grafana
    if curl -f http://localhost:3001 >/dev/null 2>&1; then
        print_success "Grafana is accessible"
    else
        print_error "Grafana is not responding"
    fi
    
    # Check Prometheus
    if curl -f http://localhost:9090 >/dev/null 2>&1; then
        print_success "Prometheus is accessible"
    else
        print_error "Prometheus is not responding"
    fi
}

# Function to show access information
show_access_info() {
    print_header "Access Information"
    
    echo -e "${CYAN}Application URLs:${NC}"
    echo -e "  📊 ${GREEN}Frontend Dashboard:${NC} http://localhost:3000"
    echo -e "  🔌 ${GREEN}Backend API:${NC} http://localhost:8080"
    echo -e "  📈 ${GREEN}Grafana:${NC} http://localhost:3001 (admin/admin)"
    echo -e "  📊 ${GREEN}Prometheus:${NC} http://localhost:9090"
    
    echo -e "\n${CYAN}API Endpoints:${NC}"
    echo -e "  🔍 ${GREEN}Health Check:${NC} http://localhost:8080/health"
    echo -e "  💰 ${GREEN}Cost Data:${NC} http://localhost:8080/api/costs/namespace/default"
    echo -e "  💡 ${GREEN}Recommendations:${NC} http://localhost:8080/api/recommendations/default"
    echo -e "  📊 ${GREEN}Metrics:${NC} http://localhost:8080/metrics"
    
    echo -e "\n${CYAN}Database Access:${NC}"
    echo -e "  🗄️ ${GREEN}PostgreSQL:${NC} localhost:5432"
    echo -e "  ⚡ ${GREEN}Redis:${NC} localhost:6379"
    
    echo -e "\n${CYAN}Sample Data:${NC}"
    echo -e "  📊 ${GREEN}Namespaces:${NC} default, production, staging, development"
    echo -e "  🏷️ ${GREEN}Time Range:${NC} Last 30 days of mock data"
    echo -e "  💰 ${GREEN}Cost Range:${NC} $100 - $5000 per namespace"
}

# Function to run quick tests
run_tests() {
    print_header "Running Quick Tests"
    
    # Test backend health
    print_status "Testing backend health..."
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        print_success "Backend health check passed"
    else
        print_error "Backend health check failed"
        return 1
    fi
    
    # Test cost API
    print_status "Testing cost API..."
    if curl -f http://localhost:8080/api/costs/namespace/default >/dev/null 2>&1; then
        print_success "Cost API test passed"
    else
        print_error "Cost API test failed"
        return 1
    fi
    
    # Test recommendations API
    print_status "Testing recommendations API..."
    if curl -f http://localhost:8080/api/recommendations/default >/dev/null 2>&1; then
        print_success "Recommendations API test passed"
    else
        print_error "Recommendations API test failed"
        return 1
    fi
    
    # Test frontend
    print_status "Testing frontend..."
    if curl -f http://localhost:3000 >/dev/null 2>&1; then
        print_success "Frontend test passed"
    else
        print_error "Frontend test failed"
        return 1
    fi
    
    print_success "All tests passed!"
}

# Function to show logs
show_logs() {
    print_header "Application Logs"
    
    cd "$SCRIPT_DIR"
    
    echo -e "${CYAN}Backend Logs:${NC}"
    docker-compose -f $DOCKER_COMPOSE_FILE logs --tail=20 backend
    
    echo -e "\n${CYAN}Frontend Logs:${NC}"
    docker-compose -f $DOCKER_COMPOSE_FILE logs --tail=20 frontend
    
    echo -e "\n${CYAN}Database Logs:${NC}"
    docker-compose -f $DOCKER_COMPOSE_FILE logs --tail=10 postgres
}

# Function to stop the application
stop_application() {
    print_header "Stopping Application"
    
    cd "$SCRIPT_DIR"
    
    print_status "Stopping all services..."
    docker-compose -f $DOCKER_COMPOSE_FILE down
    
    print_success "Application stopped"
}

# Function to clean up
cleanup() {
    print_header "Cleaning Up"
    
    cd "$SCRIPT_DIR"
    
    print_status "Stopping and removing containers..."
    docker-compose -f $DOCKER_COMPOSE_FILE down -v
    
    print_status "Removing volumes..."
    docker volume rm k8s-kost_postgres_data k8s-kost_redis_data 2>/dev/null || true
    
    print_status "Removing images..."
    docker rmi k8s-cost-optimizer:latest 2>/dev/null || true
    
    print_success "Cleanup complete"
}

# Function to restart the application
restart_application() {
    print_header "Restarting Application"
    
    stop_application
    sleep 2
    start_application
}

# Function to show help
show_help() {
    echo -e "${PURPLE}Kubernetes Cost Optimizer - Quick Test Script${NC}"
    echo ""
    echo -e "${CYAN}Usage:${NC} $0 [COMMAND]"
    echo ""
    echo -e "${CYAN}Commands:${NC}"
    echo -e "  ${GREEN}start${NC}     - Start the application with sample data"
    echo -e "  ${GREEN}stop${NC}      - Stop the application"
    echo -e "  ${GREEN}restart${NC}   - Restart the application"
    echo -e "  ${GREEN}status${NC}    - Check application status"
    echo -e "  ${GREEN}test${NC}      - Run quick tests"
    echo -e "  ${GREEN}logs${NC}      - Show application logs"
    echo -e "  ${GREEN}cleanup${NC}   - Stop and clean up everything"
    echo -e "  ${GREEN}check${NC}     - Check prerequisites"
    echo -e "  ${GREEN}help${NC}      - Show this help message"
    echo ""
    echo -e "${CYAN}Examples:${NC}"
    echo -e "  $0 start      # Start the application"
    echo -e "  $0 status     # Check if everything is running"
    echo -e "  $0 test       # Run tests to verify functionality"
    echo -e "  $0 logs       # View application logs"
    echo -e "  $0 cleanup    # Clean up everything"
}

# Main script logic
case "${1:-help}" in
    start)
        check_prerequisites
        start_application
        sleep 5
        check_status
        show_access_info
        ;;
    stop)
        stop_application
        ;;
    restart)
        restart_application
        ;;
    status)
        check_status
        show_access_info
        ;;
    test)
        run_tests
        ;;
    logs)
        show_logs
        ;;
    cleanup)
        cleanup
        ;;
    check)
        check_prerequisites
        ;;
    help|*)
        show_help
        ;;
esac 