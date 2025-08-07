# Kubernetes Deployment Guide

This guide shows you how to deploy the Kubernetes Cost Optimizer application to your Kubernetes cluster.

## Prerequisites

### Required Tools
- **kubectl** (configured for your cluster)
- **Docker** (for building images)
- **Make** (for automation)
- **Helm** (optional, for complex deployments)

### Cluster Requirements
- **Kubernetes v1.20+**
- **Metrics Server** enabled
- **Ingress Controller** (for external access)
- **Storage Class** (for persistent volumes)
- **RBAC** enabled

## Quick Deployment

### Option 1: Automated Deployment (Recommended)

```bash
# Clone the repository
git clone <your-repo-url>
cd k8s-kost

# Configure environment
cp env.example .env
# Edit .env with your cluster configuration

# Deploy everything
./deploy.sh
```

### Option 2: Manual Deployment

```bash
# Build and push images
make docker-build
make docker-push

# Deploy to cluster
make deploy
```

## Cluster-Specific Deployments

### Minikube Deployment

```bash
# Start Minikube
minikube start --driver=docker --cpus=4 --memory=8192

# Enable addons
minikube addons enable metrics-server
minikube addons enable ingress

# Deploy application
make deploy-minikube

# Access the application
minikube service k8s-cost-optimizer -n kube-system
```

### Kind (Kubernetes in Docker) Deployment

```bash
# Create Kind cluster
kind create cluster --name k8s-cost-optimizer --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 3000
  - containerPort: 30001
    hostPort: 8080
- role: worker
- role: worker
EOF

# Deploy application
make deploy-kind

# Access via port-forward
kubectl port-forward svc/k8s-cost-optimizer 3000:80 -n kube-system
kubectl port-forward svc/k8s-cost-optimizer 8080:8080 -n kube-system
```

### Docker Desktop Kubernetes

```bash
# Enable Kubernetes in Docker Desktop
# Then deploy:
make deploy-local

# Access via port-forward
kubectl port-forward svc/k8s-cost-optimizer 3000:80 -n kube-system
```

### Production Cluster (EKS, GKE, AKS)

```bash
# Configure kubectl for your cluster
aws eks update-kubeconfig --name your-cluster-name  # EKS
# or
gcloud container clusters get-credentials your-cluster-name --zone your-zone  # GKE
# or
az aks get-credentials --resource-group your-rg --name your-cluster-name  # AKS

# Deploy with production settings
make deploy-prod
```

## Detailed Deployment Steps

### Step 1: Build and Push Images

```bash
# Set your registry
export REGISTRY=your-registry.com
export VERSION=v1.0.0

# Build images
make docker-build

# Push to registry
make docker-push
```

### Step 2: Configure Environment

Create a production environment file:

```bash
# Copy and edit environment file
cp env.example .env.prod

# Edit with production values
nano .env.prod
```

**Production environment settings:**

```bash
# Database Configuration
DATABASE_URL=postgresql://k8s_cost_user:secure_password@postgresql:5432/k8s_cost_optimizer
REDIS_URL=redis://redis:6379

# Cloud Provider Configuration
AWS_ACCESS_KEY_ID=your_production_aws_key
AWS_SECRET_ACCESS_KEY=your_production_aws_secret
AWS_REGION=us-west-2

# Application Configuration
PROMETHEUS_URL=http://prometheus:9090
CLUSTER_NAME=production-cluster
NAMESPACE=kube-system

# Security
JWT_SECRET=your_secure_jwt_secret
ENCRYPTION_KEY=your_secure_encryption_key

# Performance
CACHE_TTL=900
METRICS_RETENTION_DAYS=90
MAX_CONCURRENT_REQUESTS=100
```

### Step 3: Create Namespace and RBAC

```bash
# Create namespace
kubectl create namespace kube-system

# Apply RBAC
kubectl apply -f deploy/kubernetes/rbac.yaml
```

### Step 4: Deploy Infrastructure

#### Option A: Using Helm Charts

```bash
# Add Helm repository
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Deploy PostgreSQL with TimescaleDB
helm install postgresql bitnami/postgresql \
  --namespace kube-system \
  --set postgresqlPassword=secure_password \
  --set postgresqlDatabase=k8s_cost_optimizer \
  --set postgresqlUsername=k8s_cost_user \
  --set postgresqlExtendedConf.enable_timescaledb=true

# Deploy Redis
helm install redis bitnami/redis \
  --namespace kube-system \
  --set auth.enabled=false \
  --set master.persistence.size=10Gi

# Deploy Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace kube-system \
  --set grafana.enabled=true \
  --set prometheus.prometheusSpec.retention=7d
```

#### Option B: Using Kubernetes Manifests

```bash
# Deploy PostgreSQL
kubectl apply -f deploy/kubernetes/postgresql.yaml

# Deploy Redis
kubectl apply -f deploy/kubernetes/redis.yaml

# Deploy Prometheus
kubectl apply -f deploy/kubernetes/prometheus.yaml
```

### Step 5: Deploy Application

```bash
# Create secrets
kubectl create secret generic k8s-cost-optimizer-secrets \
  --from-env-file=.env.prod \
  -n kube-system

# Deploy application
kubectl apply -f deploy/kubernetes/deployment.yaml
kubectl apply -f deploy/kubernetes/service.yaml
kubectl apply -f deploy/kubernetes/ingress.yaml

# Deploy monitoring
kubectl apply -f deploy/kubernetes/monitoring.yaml
```

### Step 6: Verify Deployment

```bash
# Check pod status
kubectl get pods -n kube-system -l app=k8s-cost-optimizer

# Check services
kubectl get svc -n kube-system -l app=k8s-cost-optimizer

# Check ingress
kubectl get ingress -n kube-system

# Check logs
kubectl logs -f deployment/k8s-cost-optimizer -n kube-system
```

## Access Methods

### Port Forwarding (Development)

```bash
# Forward frontend
kubectl port-forward svc/k8s-cost-optimizer 3000:80 -n kube-system

# Forward API
kubectl port-forward svc/k8s-cost-optimizer 8080:8080 -n kube-system

# Access at:
# Frontend: http://localhost:3000
# API: http://localhost:8080
```

### Load Balancer (Cloud)

```bash
# Get external IP
kubectl get svc k8s-cost-optimizer -n kube-system

# Access via external IP
curl http://<EXTERNAL-IP>
```

### Ingress (Production)

```bash
# Get ingress hostname
kubectl get ingress k8s-cost-optimizer -n kube-system

# Access via hostname
curl http://<INGRESS-HOSTNAME>
```

## Scaling and High Availability

### Horizontal Pod Autoscaler

```bash
# Apply HPA
kubectl apply -f deploy/kubernetes/hpa.yaml

# Check HPA status
kubectl get hpa -n kube-system
```

### Pod Disruption Budget

```bash
# Apply PDB for high availability
kubectl apply -f deploy/kubernetes/pdb.yaml
```

### Multi-Replica Deployment

```bash
# Scale to multiple replicas
kubectl scale deployment k8s-cost-optimizer --replicas=3 -n kube-system

# Or use the Makefile
make scale REPLICAS=3
```

## Monitoring and Observability

### Prometheus Integration

```bash
# Deploy Prometheus monitoring
kubectl apply -f deploy/kubernetes/monitoring.yaml

# Access Prometheus
kubectl port-forward svc/prometheus-operated 9090:9090 -n kube-system
# Then visit: http://localhost:9090
```

### Grafana Dashboards

```bash
# Deploy Grafana
kubectl apply -f monitoring/grafana.yaml

# Access Grafana
kubectl port-forward svc/grafana 3001:3000 -n kube-system
# Then visit: http://localhost:3001 (admin/admin)
```

### Application Metrics

```bash
# Check application metrics
curl http://localhost:8080/metrics

# Check custom metrics
curl http://localhost:8080/metrics | grep k8s_cost_optimizer
```

## Security Configuration

### Network Policies

```bash
# Apply network policies
kubectl apply -f deploy/kubernetes/network-policies.yaml
```

### Pod Security Standards

```bash
# Apply Pod Security Standards
kubectl apply -f deploy/kubernetes/pod-security.yaml
```

### Secrets Management

```bash
# Use external secrets manager
kubectl apply -f deploy/kubernetes/external-secrets.yaml
```

## Troubleshooting

### Common Issues

**Pods not starting:**
```bash
# Check pod events
kubectl describe pod <pod-name> -n kube-system

# Check pod logs
kubectl logs <pod-name> -n kube-system

# Check resource limits
kubectl top pods -n kube-system
```

**Database connection issues:**
```bash
# Check PostgreSQL status
kubectl get pods -n kube-system -l app=postgresql

# Check PostgreSQL logs
kubectl logs deployment/postgresql -n kube-system

# Test database connectivity
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- nc -zv postgresql 5432
```

**Image pull errors:**
```bash
# Check image pull secrets
kubectl get secrets -n kube-system

# Create image pull secret if needed
kubectl create secret docker-registry regcred \
  --docker-server=<your-registry> \
  --docker-username=<username> \
  --docker-password=<password> \
  -n kube-system
```

**Ingress not working:**
```bash
# Check ingress controller
kubectl get pods -n ingress-nginx

# Check ingress status
kubectl describe ingress k8s-cost-optimizer -n kube-system

# Check ingress logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx
```

### Debugging Commands

```bash
# Get detailed status
make status

# Check all resources
kubectl get all -n kube-system -l app=k8s-cost-optimizer

# Check events
kubectl get events -n kube-system --sort-by='.lastTimestamp'

# Check resource usage
kubectl top pods -n kube-system
kubectl top nodes

# Check storage
kubectl get pvc -n kube-system
kubectl get pv
```

## Performance Optimization

### Resource Limits

```bash
# Apply optimized resource limits
kubectl apply -f deploy/kubernetes/resource-limits.yaml
```

### Database Optimization

```bash
# Apply database optimizations
kubectl apply -f deploy/kubernetes/database-optimization.yaml
```

### Caching Strategy

```bash
# Configure Redis for optimal performance
kubectl exec -it deployment/redis -n kube-system -- redis-cli CONFIG SET maxmemory 1gb
kubectl exec -it deployment/redis -n kube-system -- redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

## Backup and Recovery

### Database Backup

```bash
# Create backup
kubectl exec deployment/postgresql -n kube-system -- pg_dump k8s_cost_optimizer > backup.sql

# Or use the Makefile
make backup-db
```

### Application Backup

```bash
# Export all resources
kubectl get all -n kube-system -l app=k8s-cost-optimizer -o yaml > app-backup.yaml

# Export secrets (base64 encoded)
kubectl get secrets k8s-cost-optimizer-secrets -n kube-system -o yaml > secrets-backup.yaml
```

## Updates and Rollbacks

### Rolling Updates

```bash
# Update to new version
make update

# Check rollout status
kubectl rollout status deployment/k8s-cost-optimizer -n kube-system

# Rollback if needed
make rollback
```

### Blue-Green Deployment

```bash
# Deploy new version alongside old
kubectl apply -f deploy/kubernetes/blue-green-deployment.yaml

# Switch traffic
kubectl patch service k8s-cost-optimizer -n kube-system -p '{"spec":{"selector":{"version":"v2"}}}'
```

## Production Checklist

Before going to production, ensure:

- [ ] **Security**: Network policies, RBAC, secrets management
- [ ] **Monitoring**: Prometheus, Grafana, alerting rules
- [ ] **Backup**: Database backup strategy
- [ ] **Scaling**: HPA, PDB, resource limits
- [ ] **SSL/TLS**: Ingress with HTTPS
- [ ] **Logging**: Centralized logging solution
- [ ] **Testing**: Load testing, chaos engineering
- [ ] **Documentation**: Runbooks, troubleshooting guides

## Next Steps

After successful deployment:

1. **Configure monitoring** - Set up dashboards and alerts
2. **Set up CI/CD** - Automate deployments
3. **Configure backup** - Implement automated backups
4. **Performance tuning** - Optimize based on usage patterns
5. **Security hardening** - Implement additional security measures

For more detailed information, see the main [README.md](README.md) and [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md). 