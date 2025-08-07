package collectors

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetricsCollector struct {
	k8sClient     kubernetes.Interface
	metricsClient versioned.Interface
	promClient    v1.API
	db            *sql.DB
	log           *logrus.Logger
}

func NewMetricsCollector(k8sClient kubernetes.Interface, db *sql.DB) *MetricsCollector {
	// Initialize Prometheus client
	promClient, err := api.NewClient(api.Config{
		Address: "http://prometheus:9090",
	})
	if err != nil {
		logrus.Warnf("Failed to initialize Prometheus client: %v", err)
	}

	var promAPI v1.API
	if promClient != nil {
		promAPI = v1.NewAPI(promClient)
	}

	// Initialize metrics client
	metricsClient, err := versioned.NewForConfig(k8sClient.RESTClient().Config())
	if err != nil {
		logrus.Warnf("Failed to initialize metrics client: %v", err)
	}

	return &MetricsCollector{
		k8sClient:     k8sClient,
		metricsClient: metricsClient,
		promClient:    promAPI,
		db:            db,
		log:           logrus.New(),
	}
}

func (mc *MetricsCollector) CollectNamespaceMetrics(ctx context.Context) error {
	if mc.promClient == nil {
		return fmt.Errorf("Prometheus client not available")
	}

	// Query CPU usage by namespace
	cpuQuery := `sum by (namespace) (
		rate(container_cpu_usage_seconds_total[5m]) * 1000
	)`
	
	result, warnings, err := mc.promClient.Query(ctx, cpuQuery, time.Now())
	if err != nil {
		return fmt.Errorf("querying CPU metrics: %w", err)
	}
	
	if len(warnings) > 0 {
		mc.log.Warnf("Prometheus warnings: %v", warnings)
	}

	// Process CPU results
	if err := mc.processNamespaceMetrics(result, "cpu_millicores"); err != nil {
		return fmt.Errorf("processing CPU metrics: %w", err)
	}

	// Query memory usage by namespace
	memQuery := `sum by (namespace) (
		container_memory_working_set_bytes
	)`
	
	memResult, _, err := mc.promClient.Query(ctx, memQuery, time.Now())
	if err != nil {
		return fmt.Errorf("querying memory metrics: %w", err)
	}

	// Process memory results
	if err := mc.processNamespaceMetrics(memResult, "memory_bytes"); err != nil {
		return fmt.Errorf("processing memory metrics: %w", err)
	}

	// Query storage usage by namespace
	storageQuery := `sum by (namespace, persistentvolumeclaim) (
		kubelet_volume_stats_used_bytes
	)`
	
	storageResult, _, err := mc.promClient.Query(ctx, storageQuery, time.Now())
	if err != nil {
		mc.log.Warnf("Failed to query storage metrics: %v", err)
	} else {
		if err := mc.processStorageMetrics(storageResult); err != nil {
			mc.log.Warnf("Failed to process storage metrics: %v", err)
		}
	}

	return nil
}

func (mc *MetricsCollector) processNamespaceMetrics(result model.Value, metricType string) error {
	matrix, ok := result.(model.Matrix)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	timestamp := time.Now()
	
	for _, sample := range matrix {
		namespace := string(sample.Metric["namespace"])
		if namespace == "" {
			continue
		}

		// Get the latest value
		if len(sample.Values) == 0 {
			continue
		}
		
		value := float64(sample.Values[len(sample.Values)-1].Value)

		// Store in database
		_, err := mc.db.Exec(`
			INSERT INTO namespace_metrics 
			(namespace, metric_type, value, timestamp) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (namespace, metric_type, timestamp) 
			DO UPDATE SET value = $3
		`, namespace, metricType, value, timestamp)
		
		if err != nil {
			return fmt.Errorf("storing %s metrics for namespace %s: %w", metricType, namespace, err)
		}
	}

	return nil
}

func (mc *MetricsCollector) processStorageMetrics(result model.Value) error {
	matrix, ok := result.(model.Matrix)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	timestamp := time.Now()
	
	for _, sample := range matrix {
		namespace := string(sample.Metric["namespace"])
		pvc := string(sample.Metric["persistentvolumeclaim"])
		
		if namespace == "" || pvc == "" {
			continue
		}

		if len(sample.Values) == 0 {
			continue
		}
		
		value := float64(sample.Values[len(sample.Values)-1].Value)

		// Store storage metrics
		_, err := mc.db.Exec(`
			INSERT INTO storage_metrics 
			(namespace, pvc_name, used_bytes, timestamp) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (namespace, pvc_name, timestamp) 
			DO UPDATE SET used_bytes = $3
		`, namespace, pvc, value, timestamp)
		
		if err != nil {
			return fmt.Errorf("storing storage metrics: %w", err)
		}
	}

	return nil
}

func (mc *MetricsCollector) CollectPodMetrics(ctx context.Context) error {
	if mc.metricsClient == nil {
		return fmt.Errorf("metrics client not available")
	}

	// Get pod metrics from Metrics Server
	podMetricsList, err := mc.metricsClient.MetricsV1beta1().
		PodMetricses("").
		List(ctx, metav1.ListOptions{})
	
	if err != nil {
		return fmt.Errorf("fetching pod metrics: %w", err)
	}

	timestamp := time.Now()
	
	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			cpu := container.Usage.Cpu().MilliValue()
			memory := container.Usage.Memory().Value()
			
			// Store detailed pod-level metrics
			_, err = mc.db.Exec(`
				INSERT INTO pod_metrics 
				(namespace, pod_name, container_name, cpu_millicores, memory_bytes, timestamp)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (namespace, pod_name, container_name, timestamp) 
				DO UPDATE SET 
					cpu_millicores = $4,
					memory_bytes = $5
			`, podMetrics.Namespace, podMetrics.Name, container.Name, 
			   cpu, memory, timestamp)
			
			if err != nil {
				mc.log.Warnf("Failed to store pod metrics for %s/%s/%s: %v", 
					podMetrics.Namespace, podMetrics.Name, container.Name, err)
			}
		}
	}

	return nil
}

func (mc *MetricsCollector) CollectNodeMetrics(ctx context.Context) error {
	if mc.metricsClient == nil {
		return fmt.Errorf("metrics client not available")
	}

	// Get node metrics from Metrics Server
	nodeMetricsList, err := mc.metricsClient.MetricsV1beta1().
		NodeMetricses().
		List(ctx, metav1.ListOptions{})
	
	if err != nil {
		return fmt.Errorf("fetching node metrics: %w", err)
	}

	timestamp := time.Now()
	
	for _, nodeMetrics := range nodeMetricsList.Items {
		cpu := nodeMetrics.Usage.Cpu().MilliValue()
		memory := nodeMetrics.Usage.Memory().Value()
		
		// Store node metrics
		_, err = mc.db.Exec(`
			INSERT INTO node_metrics 
			(node_name, cpu_millicores, memory_bytes, timestamp)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (node_name, timestamp) 
			DO UPDATE SET 
				cpu_millicores = $2,
				memory_bytes = $3
		`, nodeMetrics.Name, cpu, memory, timestamp)
		
		if err != nil {
			mc.log.Warnf("Failed to store node metrics for %s: %v", nodeMetrics.Name, err)
		}
	}

	return nil
}

func (mc *MetricsCollector) CollectResourceRequests(ctx context.Context) error {
	// Get all namespaces
	namespaces, err := mc.k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing namespaces: %w", err)
	}

	timestamp := time.Now()

	for _, namespace := range namespaces.Items {
		// Get all pods in the namespace
		pods, err := mc.k8sClient.CoreV1().Pods(namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			mc.log.Warnf("Failed to list pods in namespace %s: %v", namespace.Name, err)
			continue
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				cpuRequest := container.Resources.Requests.Cpu().MilliValue()
				cpuLimit := container.Resources.Limits.Cpu().MilliValue()
				memoryRequest := container.Resources.Requests.Memory().Value()
				memoryLimit := container.Resources.Limits.Memory().Value()

				// Store resource requests/limits
				_, err = mc.db.Exec(`
					INSERT INTO resource_requests 
					(namespace, pod_name, container_name, cpu_request, cpu_limit, memory_request, memory_limit, timestamp)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
					ON CONFLICT (namespace, pod_name, container_name, timestamp) 
					DO UPDATE SET 
						cpu_request = $4,
						cpu_limit = $5,
						memory_request = $6,
						memory_limit = $7
				`, namespace.Name, pod.Name, container.Name, 
				   cpuRequest, cpuLimit, memoryRequest, memoryLimit, timestamp)
				
				if err != nil {
					mc.log.Warnf("Failed to store resource requests for %s/%s/%s: %v", 
						namespace.Name, pod.Name, container.Name, err)
				}
			}
		}
	}

	return nil
}

func (mc *MetricsCollector) CollectCosts(ctx context.Context, costProvider interface{}) error {
	// This method will be implemented to collect costs from cloud providers
	// For now, we'll use mock data
	return mc.collectMockCosts(ctx)
}

func (mc *MetricsCollector) collectMockCosts(ctx context.Context) error {
	// Get all namespaces
	namespaces, err := mc.k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing namespaces: %w", err)
	}

	timestamp := time.Now()

	for _, namespace := range namespaces.Items {
		// Calculate mock costs based on resource usage
		var computeCost, storageCost, networkCost, otherCost float64

		// Query recent resource usage
		var cpuUsage, memoryUsage float64
		err = mc.db.QueryRow(`
			SELECT AVG(value) FROM namespace_metrics 
			WHERE namespace = $1 AND metric_type = 'cpu_millicores' 
			AND timestamp > NOW() - INTERVAL '1 hour'
		`, namespace.Name).Scan(&cpuUsage)
		if err != nil && err != sql.ErrNoRows {
			mc.log.Warnf("Failed to get CPU usage for %s: %v", namespace.Name, err)
		}

		err = mc.db.QueryRow(`
			SELECT AVG(value) FROM namespace_metrics 
			WHERE namespace = $1 AND metric_type = 'memory_bytes' 
			AND timestamp > NOW() - INTERVAL '1 hour'
		`, namespace.Name).Scan(&memoryUsage)
		if err != nil && err != sql.ErrNoRows {
			mc.log.Warnf("Failed to get memory usage for %s: %v", namespace.Name, err)
		}

		// Calculate mock costs (simplified pricing model)
		computeCost = (cpuUsage * 0.00001) + (memoryUsage * 0.00000001) // $0.00001 per millicore, $0.00000001 per byte
		storageCost = computeCost * 0.2  // 20% of compute cost
		networkCost = computeCost * 0.1  // 10% of compute cost
		otherCost = computeCost * 0.05   // 5% of compute cost

		// Store costs
		_, err = mc.db.Exec(`
			INSERT INTO namespace_costs 
			(namespace, compute_cost, storage_cost, network_cost, other_cost, timestamp)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (namespace, timestamp) 
			DO UPDATE SET 
				compute_cost = $2,
				storage_cost = $3,
				network_cost = $4,
				other_cost = $5
		`, namespace.Name, computeCost, storageCost, networkCost, otherCost, timestamp)
		
		if err != nil {
			mc.log.Warnf("Failed to store costs for namespace %s: %v", namespace.Name, err)
		}
	}

	return nil
}

// Helper method to get current resource allocation
func (mc *MetricsCollector) GetCurrentAllocation(namespace, podName, containerName string) (map[string]float64, error) {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit float64

	err := mc.db.QueryRow(`
		SELECT cpu_request, cpu_limit, memory_request, memory_limit 
		FROM resource_requests 
		WHERE namespace = $1 AND pod_name = $2 AND container_name = $3
		ORDER BY timestamp DESC LIMIT 1
	`, namespace, podName, containerName).Scan(&cpuRequest, &cpuLimit, &memoryRequest, &memoryLimit)

	if err != nil {
		return nil, fmt.Errorf("getting current allocation: %w", err)
	}

	return map[string]float64{
		"cpu_request":    cpuRequest,
		"cpu_limit":      cpuLimit,
		"memory_request": memoryRequest,
		"memory_limit":   memoryLimit,
	}, nil
} 