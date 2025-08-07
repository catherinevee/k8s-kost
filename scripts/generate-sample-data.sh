#!/bin/bash

# Generate Sample Data for Local Development
# This script creates mock data for testing the Kubernetes Cost Optimizer

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Check if database is accessible
check_database() {
    log "Checking database connection..."
    
    if ! docker exec k8s-cost-postgres pg_isready -U k8s_cost_user -d k8s_cost_optimizer > /dev/null 2>&1; then
        echo "Error: Cannot connect to PostgreSQL. Make sure the database is running."
        echo "Run: docker-compose -f docker-compose.local.yml up -d postgres"
        exit 1
    fi
    
    success "Database connection verified"
}

# Generate mock Kubernetes metrics
generate_metrics() {
    log "Generating mock Kubernetes metrics..."
    
    # Generate namespace metrics for the last 7 days
    for i in {0..6}; do
        date=$(date -d "$i days ago" +%Y-%m-%d)
        
        # Generate metrics for different namespaces
        for namespace in "default" "production" "staging" "development" "monitoring"; do
            # CPU metrics (millicores)
            cpu_usage=$((RANDOM % 500 + 100))
            docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
                INSERT INTO namespace_metrics (namespace, metric_type, value, timestamp)
                VALUES ('$namespace', 'cpu_millicores', $cpu_usage, '$date 12:00:00+00')
                ON CONFLICT (namespace, metric_type, timestamp) DO NOTHING;
            " > /dev/null 2>&1
            
            # Memory metrics (bytes)
            memory_usage=$((RANDOM % 1073741824 + 268435456))  # 256MB to 1.25GB
            docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
                INSERT INTO namespace_metrics (namespace, metric_type, value, timestamp)
                VALUES ('$namespace', 'memory_bytes', $memory_usage, '$date 12:00:00+00')
                ON CONFLICT (namespace, metric_type, timestamp) DO NOTHING;
            " > /dev/null 2>&1
        done
    done
    
    success "Generated namespace metrics"
}

# Generate mock pod metrics
generate_pod_metrics() {
    log "Generating mock pod metrics..."
    
    # Sample pods for different namespaces
    declare -A pods=(
        ["default"]="nginx-deployment web-server api-gateway"
        ["production"]="user-service payment-service order-service"
        ["staging"]="test-api mock-service staging-web"
        ["development"]="dev-api local-service debug-pod"
        ["monitoring"]="prometheus grafana alertmanager"
    )
    
    for namespace in "${!pods[@]}"; do
        for pod in ${pods[$namespace]}; do
            for container in "main" "sidecar"; do
                # Generate metrics for the last 3 days
                for i in {0..2}; do
                    date=$(date -d "$i days ago" +%Y-%m-%d)
                    
                    # CPU usage (millicores)
                    cpu_usage=$((RANDOM % 200 + 50))
                    
                    # Memory usage (bytes)
                    memory_usage=$((RANDOM % 536870912 + 67108864))  # 64MB to 512MB
                    
                    docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
                        INSERT INTO pod_metrics (namespace, pod_name, container_name, cpu_millicores, memory_bytes, timestamp)
                        VALUES ('$namespace', '$pod', '$container', $cpu_usage, $memory_usage, '$date 12:00:00+00')
                        ON CONFLICT (namespace, pod_name, container_name, timestamp) DO NOTHING;
                    " > /dev/null 2>&1
                done
            done
        done
    done
    
    success "Generated pod metrics"
}

# Generate mock cost data
generate_costs() {
    log "Generating mock cost data..."
    
    # Generate costs for the last 30 days
    for i in {0..29}; do
        date=$(date -d "$i days ago" +%Y-%m-%d)
        
        for namespace in "default" "production" "staging" "development" "monitoring"; do
            # Generate realistic cost breakdowns
            compute_cost=$(echo "scale=4; $((RANDOM % 50 + 10)) / 100" | bc)
            storage_cost=$(echo "scale=4; $((RANDOM % 20 + 5)) / 100" | bc)
            network_cost=$(echo "scale=4; $((RANDOM % 15 + 3)) / 100" | bc)
            other_cost=$(echo "scale=4; $((RANDOM % 10 + 1)) / 100" | bc)
            
            docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
                INSERT INTO namespace_costs (namespace, compute_cost, storage_cost, network_cost, other_cost, timestamp)
                VALUES ('$namespace', $compute_cost, $storage_cost, $network_cost, $other_cost, '$date 00:00:00+00')
                ON CONFLICT (namespace, timestamp) DO NOTHING;
            " > /dev/null 2>&1
        done
    done
    
    success "Generated cost data"
}

# Generate mock recommendations
generate_recommendations() {
    log "Generating mock recommendations..."
    
    # Sample recommendations for different pods
    declare -A recommendations=(
        ["nginx-deployment"]="CPU: 200m -> 150m, Memory: 256Mi -> 128Mi"
        ["web-server"]="CPU: 500m -> 300m, Memory: 512Mi -> 256Mi"
        ["api-gateway"]="CPU: 300m -> 200m, Memory: 384Mi -> 192Mi"
        ["user-service"]="CPU: 400m -> 250m, Memory: 512Mi -> 256Mi"
        ["payment-service"]="CPU: 600m -> 400m, Memory: 768Mi -> 512Mi"
    )
    
    for pod in "${!recommendations[@]}"; do
        # Parse recommendation values
        cpu_current=$(echo "${recommendations[$pod]}" | grep -o 'CPU: [0-9]*m' | cut -d' ' -f2 | sed 's/m//')
        cpu_recommended=$(echo "${recommendations[$pod]}" | grep -o '-> [0-9]*m' | cut -d' ' -f2 | sed 's/m//')
        memory_current=$(echo "${recommendations[$pod]}" | grep -o 'Memory: [0-9]*Mi' | cut -d' ' -f2 | sed 's/Mi//')
        memory_recommended=$(echo "${recommendations[$pod]}" | grep -o '-> [0-9]*Mi' | cut -d' ' -f2 | sed 's/Mi//')
        
        # Convert memory to bytes
        memory_current_bytes=$((memory_current * 1048576))
        memory_recommended_bytes=$((memory_recommended * 1048576))
        
        # Calculate potential savings
        cpu_savings=$(echo "scale=4; ($cpu_current - $cpu_recommended) * 0.00001 * 24 * 30" | bc)
        memory_savings=$(echo "scale=4; ($memory_current_bytes - $memory_recommended_bytes) * 0.00000001 * 24 * 30" | bc)
        total_savings=$(echo "scale=4; $cpu_savings + $memory_savings" | bc)
        
        # Insert CPU recommendation
        docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
            INSERT INTO recommendations (namespace, pod_name, container_name, resource_type, current_request, current_limit, recommended_request, recommended_limit, potential_savings, confidence, created_at)
            VALUES ('default', '$pod', 'main', 'CPU', $cpu_current, $((cpu_current * 2)), $cpu_recommended, $((cpu_recommended * 2)), $cpu_savings, 0.85, NOW())
            ON CONFLICT DO NOTHING;
        " > /dev/null 2>&1
        
        # Insert Memory recommendation
        docker exec k8s-cost-postgres psql -U k8s_cost_user -d k8s_cost_optimizer -c "
            INSERT INTO recommendations (namespace, pod_name, container_name, resource_type, current_request, current_limit, recommended_request, recommended_limit, potential_savings, confidence, created_at)
            VALUES ('default', '$pod', 'main', 'Memory', $memory_current_bytes, $((memory_current_bytes * 2)), $memory_recommended_bytes, $((memory_recommended_bytes * 2)), $memory_savings, 0.90, NOW())
            ON CONFLICT DO NOTHING;
        " > /dev/null 2>&1
    done
    
    success "Generated recommendations"
}

# Main execution
main() {
    echo "=========================================="
    echo "Generating Sample Data for Local Development"
    echo "=========================================="
    echo ""
    
    # Check prerequisites
    check_database
    
    # Generate different types of data
    generate_metrics
    generate_pod_metrics
    generate_costs
    generate_recommendations
    
    echo ""
    echo "=========================================="
    success "Sample data generation completed!"
    echo "=========================================="
    echo ""
    echo "Generated data includes:"
    echo "- 7 days of namespace metrics"
    echo "- 3 days of pod metrics"
    echo "- 30 days of cost data"
    echo "- Sample optimization recommendations"
    echo ""
    echo "You can now access the application at:"
    echo "- Frontend: http://localhost:3000"
    echo "- Backend API: http://localhost:8080"
    echo "- Grafana: http://localhost:3001 (admin/admin)"
    echo "- Prometheus: http://localhost:9090"
}

# Run main function
main "$@" 