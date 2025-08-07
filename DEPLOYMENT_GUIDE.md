# Kubernetes Cost Optimizer - Deployment Guide

## Prerequisites

Before deploying the application, ensure you have the following:

### 1. Kubernetes Cluster
- A running Kubernetes cluster (v1.20+)
- `kubectl` configured and connected to your cluster
- Access to create namespaces, deployments, and RBAC resources

### 2. Infrastructure Components
- **PostgreSQL Database** (with TimescaleDB extension)
- **Redis** for caching
- **Prometheus** for metrics collection
- **Container Registry** (Docker Hub, ECR, GCR, or private registry)

### 3. Cloud Provider Access
- AWS credentials (for Cost Explorer API)
- Azure credentials (for Cost Management API)
- GCP credentials (for Billing API)

### 4. Development Tools
- Docker
- Go 1.19+
- Node.js 16+
- Make

## Quick Start Deployment

### Step 1: Clone and Setup
```bash
# Navigate to the project directory
cd k8s-kost

# Install dependencies
make install-deps
```

### Step 2: Configure Environment

Create a `.env` file in the root directory:
```bash
# Database Configuration
DATABASE_URL=postgresql://username:password@host:5432/k8s_cost_optimizer
REDIS_URL=redis://redis-host:6379

# Cloud Provider Configuration
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_REGION=us-west-2

# Application Configuration
PROMETHEUS_URL=http://prometheus:9090
CLUSTER_NAME=your-cluster-name
NAMESPACE=kube-system

# Docker Registry
REGISTRY=your-registry.com
VERSION=v1.0.0
```

### Step 3: Build and Push Images
```bash
# Build Docker images
make docker-build

# Push to registry
make docker-push
```

### Step 4: Deploy to Kubernetes
```bash
# Deploy all components
make deploy
```

## Detailed Deployment Steps

### 1. Database Setup

#### Option A: External PostgreSQL
```bash
# Create database
createdb k8s_cost_optimizer

# Enable TimescaleDB extension
psql -d k8s_cost_optimizer -c "CREATE EXTENSION IF NOT EXISTS timescaledb;"

# Run migrations
make setup-db
```

#### Option B: Deploy PostgreSQL in Kubernetes
```yaml
# deploy/postgresql.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgresql
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      containers:
      - name: postgresql
        image: timescale/timescaledb:latest-pg14
        env:
        - name: POSTGRES_DB
          value: k8s_cost_optimizer
        - name: POSTGRES_USER
          value: k8s_cost_user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgresql-secrets
              key: password
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgresql-data
          mountPath: /var/lib/postgresql/data
      volumes:
      - name: postgresql-data
        persistentVolumeClaim:
          claimName: postgresql-pvc
```

### 2. Redis Setup
```yaml
# deploy/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: redis-data
          mountPath: /data
      volumes:
      - name: redis-data
        persistentVolumeClaim:
          claimName: redis-pvc
```

### 3. Secrets Configuration
```bash
# Create secrets
kubectl create secret generic k8s-cost-optimizer-secrets \
  --from-literal=database-url="postgresql://k8s_cost_user:password@postgresql:5432/k8s_cost_optimizer" \
  --from-literal=redis-url="redis://redis:6379" \
  --from-literal=aws-access-key="your_aws_access_key" \
  --from-literal=aws-secret-key="your_aws_secret_key" \
  -n kube-system
```

### 4. Application Deployment
```bash
# Deploy the application
kubectl apply -f deploy/kubernetes/deployment.yaml
kubectl apply -f deploy/kubernetes/service.yaml
kubectl apply -f deploy/kubernetes/ingress.yaml
```

### 5. Monitoring Setup
```bash
# Deploy Prometheus rules
kubectl apply -f monitoring/prometheus-rules.yaml

# Deploy Grafana dashboard
kubectl apply -f monitoring/grafana-dashboard.yaml
```

## Configuration Options

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `REDIS_URL` | Redis connection string | Required |
| `PROMETHEUS_URL` | Prometheus server URL | `http://prometheus:9090` |
| `AWS_ACCESS_KEY_ID` | AWS access key | Required for AWS |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | Required for AWS |
| `CLUSTER_NAME` | Kubernetes cluster name | Required |
| `LOG_LEVEL` | Logging level | `info` |
| `API_PORT` | API server port | `8080` |
| `FRONTEND_PORT` | Frontend server port | `3000` |

### Resource Limits

The application includes resource requests and limits:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Accessing the Application

### Port Forwarding (Development)
```bash
# Forward frontend port
kubectl port-forward svc/k8s-cost-optimizer 3000:80 -n kube-system

# Forward API port
kubectl port-forward svc/k8s-cost-optimizer 8080:8080 -n kube-system
```

### Ingress (Production)
```bash
# Get the external IP
kubectl get ingress k8s-cost-optimizer -n kube-system

# Access the application
curl http://<EXTERNAL-IP>
```

## Monitoring and Troubleshooting

### Check Application Status
```bash
# Check pod status
kubectl get pods -n kube-system -l app=k8s-cost-optimizer

# Check service status
kubectl get svc -n kube-system -l app=k8s-cost-optimizer

# Check logs
kubectl logs -f deployment/k8s-cost-optimizer -n kube-system
```

### Common Issues

#### 1. Database Connection Issues
```bash
# Check database connectivity
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- nc -zv postgresql 5432

# Check database logs
kubectl logs deployment/postgresql -n kube-system
```

#### 2. Prometheus Connection Issues
```bash
# Check Prometheus connectivity
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- curl -f http://prometheus:9090/api/v1/status/config
```

#### 3. Cloud Provider API Issues
```bash
# Check AWS credentials
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- aws sts get-caller-identity
```

### Health Checks

The application provides health check endpoints:

```bash
# Health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready

# Metrics endpoint
curl http://localhost:8080/metrics
```

## Scaling and Updates

### Horizontal Scaling
```bash
# Scale the application
kubectl scale deployment k8s-cost-optimizer --replicas=3 -n kube-system

# Or use the Makefile
make scale REPLICAS=3
```

### Rolling Updates
```bash
# Update to new version
make update

# Rollback if needed
make rollback
```

### Auto-scaling
The application includes HPA (Horizontal Pod Autoscaler):

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: k8s-cost-optimizer-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: k8s-cost-optimizer
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Backup and Recovery

### Database Backup
```bash
# Create backup
pg_dump k8s_cost_optimizer > backup_$(date +%Y%m%d_%H%M%S).sql

# Or use the Makefile
make backup-db
```

### Application Backup
```bash
# Export Kubernetes resources
kubectl get all -n kube-system -l app=k8s-cost-optimizer -o yaml > app_backup.yaml

# Export secrets (base64 encoded)
kubectl get secrets k8s-cost-optimizer-secrets -n kube-system -o yaml > secrets_backup.yaml
```

## Security Considerations

### RBAC Configuration
The application requires specific RBAC permissions:

```yaml
# ClusterRole for Kubernetes API access
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-cost-optimizer
rules:
- apiGroups: [""]
  resources: ["pods", "nodes", "namespaces", "services"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list"]
```

### Network Policies
```yaml
# Network policy for security
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: k8s-cost-optimizer-network-policy
spec:
  podSelector:
    matchLabels:
      app: k8s-cost-optimizer
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 6379
```

## Performance Optimization

### Database Optimization
```sql
-- Create indexes for better performance
CREATE INDEX CONCURRENTLY idx_pod_metrics_timestamp ON pod_metrics(timestamp);
CREATE INDEX CONCURRENTLY idx_namespace_costs_timestamp ON namespace_costs(timestamp);

-- Set up partitioning for large tables
SELECT create_hypertable('pod_metrics', 'timestamp', chunk_time_interval => INTERVAL '1 day');
```

### Caching Strategy
```bash
# Configure Redis for optimal performance
kubectl exec -it deployment/redis -n kube-system -- redis-cli CONFIG SET maxmemory 256mb
kubectl exec -it deployment/redis -n kube-system -- redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

## Support and Maintenance

### Log Management
```bash
# View application logs
kubectl logs -f deployment/k8s-cost-optimizer -n kube-system

# View logs with timestamps
kubectl logs deployment/k8s-cost-optimizer -n kube-system --timestamps

# View logs for specific container
kubectl logs deployment/k8s-cost-optimizer -c backend -n kube-system
```

### Metrics and Monitoring
```bash
# Check Prometheus metrics
curl http://localhost:8080/metrics

# Check custom metrics
curl http://localhost:8080/metrics | grep k8s_cost_optimizer
```

### Updates and Maintenance
```bash
# Update application
make update

# Update dependencies
make install-deps

# Run security scans
make security-scan

# Run tests
make test
```

## Troubleshooting Checklist

- [ ] Kubernetes cluster is accessible
- [ ] Database is running and accessible
- [ ] Redis is running and accessible
- [ ] Prometheus is running and accessible
- [ ] Cloud provider credentials are configured
- [ ] Docker images are built and pushed
- [ ] Secrets are created
- [ ] RBAC permissions are granted
- [ ] Network policies allow traffic
- [ ] Health checks are passing
- [ ] Metrics are being collected
- [ ] Logs show no errors

For additional support, check the application logs and Prometheus metrics for detailed error information. 