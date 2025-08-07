# Quick Start Guide - Kubernetes Cost Optimizer

## Deploy in 5 Minutes

### Prerequisites
- Kubernetes cluster (v1.20+)
- `kubectl` configured
- Docker installed
- Go 1.19+ and Node.js 16+

### Step 1: Clone and Setup
```bash
cd k8s-kost
make install-deps
```

### Step 2: Configure Environment
```bash
# Copy the example environment file
cp env.example .env

# Edit .env with your configuration
nano .env
```

**Required settings in `.env`:**
```bash
# Database (use defaults for local deployment)
DATABASE_URL=postgresql://k8s_cost_user:password@postgresql:5432/k8s_cost_optimizer

# AWS credentials (required for cost data)
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_REGION=us-west-2

# Cluster name
CLUSTER_NAME=your-cluster-name
```

### Step 3: Deploy Everything
```bash
# Run the automated deployment script
./deploy.sh
```

### Step 4: Access the Application
```bash
# Port forward to access the application
kubectl port-forward svc/k8s-cost-optimizer 3000:80 -n kube-system

# Open in browser: http://localhost:3000
```

## What You Get

After deployment, you'll have:

- **Cost Dashboard**: Real-time cost breakdown by namespace
- **Optimization Recommendations**: Resource rightsizing suggestions
- **Cost Predictions**: Future cost forecasting
- **Resource Monitoring**: CPU, memory, and storage usage
- **Alerts**: Cost and resource waste notifications

## Customization

### Change Namespace
```bash
./deploy.sh -n my-custom-namespace
```

### Use Custom Registry
```bash
REGISTRY=my-registry.com ./deploy.sh
```

### Deploy Specific Version
```bash
VERSION=v1.1.0 ./deploy.sh
```

## First Steps

1. **View Costs**: Navigate to the Overview tab to see current costs
2. **Check Recommendations**: Go to Recommendations tab for optimization suggestions
3. **Apply Changes**: Use the "Apply" buttons to implement recommendations
4. **Set Alerts**: Configure cost thresholds in the monitoring section

## Troubleshooting

### Check Status
```bash
# View application status
kubectl get pods -n kube-system -l app=k8s-cost-optimizer

# Check logs
kubectl logs -f deployment/k8s-cost-optimizer -n kube-system
```

### Common Issues

**Database Connection Error:**
```bash
# Check if PostgreSQL is running
kubectl get pods -n kube-system -l app=postgresql
```

**AWS Credentials Error:**
```bash
# Verify AWS credentials
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- aws sts get-caller-identity
```

**Prometheus Connection Error:**
```bash
# Check Prometheus connectivity
kubectl exec -it deployment/k8s-cost-optimizer -n kube-system -- curl -f http://prometheus:9090/api/v1/status/config
```

## Next Steps

- **Scale Up**: Add more replicas for high availability
- **Custom Alerts**: Configure cost thresholds and notifications
- **Data Retention**: Adjust metrics retention policies
- **Backup**: Set up automated database backups
- **Security**: Configure network policies and RBAC

## Support

- **Documentation**: See `DEPLOYMENT_GUIDE.md` for detailed instructions
- **Issues**: Check application logs for error details
- **Monitoring**: Use Prometheus and Grafana for system monitoring

---

**Need Help?** Check the full `DEPLOYMENT_GUIDE.md` for comprehensive deployment instructions and troubleshooting. 