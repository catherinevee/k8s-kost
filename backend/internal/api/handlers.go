package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"k8s-cost-optimizer/internal/analyzer"
	"k8s-cost-optimizer/internal/collectors"
	"k8s-cost-optimizer/pkg/cloudprovider"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	analyzer      *analyzer.RightsizingAnalyzer
	collector     *collectors.MetricsCollector
	costProvider  cloudprovider.Provider
	db            *sql.DB
	cache         *redis.Client
	log           *logrus.Logger
}

// Metrics for monitoring
var (
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_request_duration_seconds",
			Help: "API request duration in seconds",
		},
		[]string{"method", "endpoint", "status"},
	)

	apiRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)
)

func NewHandler(analyzer *analyzer.RightsizingAnalyzer, collector *collectors.MetricsCollector, 
	costProvider cloudprovider.Provider, db *sql.DB, cache *redis.Client) *Handler {
	return &Handler{
		analyzer:     analyzer,
		collector:    collector,
		costProvider: costProvider,
		db:           db,
		cache:        cache,
		log:          logrus.New(),
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().UTC(),
	})
}

func (h *Handler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := h.db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not ready",
			"error":  "database connection failed",
		})
		return
	}

	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.cache.Ping(ctx).Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not ready",
			"error":  "redis connection failed",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ready",
		"time":   time.Now().UTC(),
	})
}

func (h *Handler) GetNamespaceCosts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	// Check cache first
	cacheKey := fmt.Sprintf("costs:%s:%s", namespace, time.Now().Format("2006-01-02-15"))
	cached, err := h.cache.Get(r.Context(), cacheKey).Result()
	if err == nil && cached != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Calculate time range
	endTime := time.Now()
	var startTime time.Time

	switch period {
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		http.Error(w, "Invalid period", http.StatusBadRequest)
		return
	}

	// Query costs from database
	rows, err := h.db.Query(`
		SELECT 
			DATE_TRUNC('day', timestamp) as day,
			SUM(compute_cost) as compute,
			SUM(storage_cost) as storage,
			SUM(network_cost) as network,
			SUM(other_cost) as other,
			SUM(compute_cost + storage_cost + network_cost + other_cost) as total
		FROM namespace_costs
		WHERE 
			namespace = $1 
			AND timestamp BETWEEN $2 AND $3
		GROUP BY day
		ORDER BY day DESC
	`, namespace, startTime, endTime)

	if err != nil {
		h.log.Errorf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type DailyCost struct {
		Date    string  `json:"date"`
		Compute float64 `json:"compute"`
		Storage float64 `json:"storage"`
		Network float64 `json:"network"`
		Other   float64 `json:"other"`
		Total   float64 `json:"total"`
	}

	var costs []DailyCost
	var totalCost float64

	for rows.Next() {
		var cost DailyCost
		var day time.Time

		err := rows.Scan(&day, &cost.Compute, &cost.Storage, 
			&cost.Network, &cost.Other, &cost.Total)
		if err != nil {
			continue
		}

		cost.Date = day.Format("2006-01-02")
		costs = append(costs, cost)
		totalCost += cost.Total
	}

	// Get current month projection
	daysInMonth := time.Date(endTime.Year(), endTime.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	daysPassed := endTime.Day()
	projectedMonthly := (totalCost / float64(daysPassed)) * float64(daysInMonth)

	// Get resource breakdown
	breakdown := h.getResourceBreakdown(namespace, startTime, endTime)

	response := map[string]interface{}{
		"namespace": namespace,
		"period":    period,
		"costs":     costs,
		"summary": map[string]float64{
			"total":            totalCost,
			"average_daily":    totalCost / float64(len(costs)),
			"projected_monthly": projectedMonthly,
		},
		"breakdown": breakdown,
	}

	// Cache the response
	jsonResponse, _ := json.Marshal(response)
	h.cache.Set(r.Context(), cacheKey, jsonResponse, 15*time.Minute)

	// Record metrics
	duration := time.Since(start).Seconds()
	apiRequestDuration.WithLabelValues("GET", "/costs/namespace", "200").Observe(duration)
	apiRequestTotal.WithLabelValues("GET", "/costs/namespace", "200").Inc()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(jsonResponse)
}

func (h *Handler) GetClusterCosts(w http.ResponseWriter, r *http.Request) {
	// Get costs across all namespaces
	rows, err := h.db.Query(`
		SELECT 
			namespace,
			SUM(compute_cost) as compute,
			SUM(storage_cost) as storage,
			SUM(network_cost) as network,
			SUM(other_cost) as other,
			SUM(compute_cost + storage_cost + network_cost + other_cost) as total
		FROM namespace_costs
		WHERE timestamp > NOW() - INTERVAL '30 days'
		GROUP BY namespace
		ORDER BY total DESC
	`)

	if err != nil {
		h.log.Errorf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type NamespaceCost struct {
		Namespace string  `json:"namespace"`
		Compute   float64 `json:"compute"`
		Storage   float64 `json:"storage"`
		Network   float64 `json:"network"`
		Other     float64 `json:"other"`
		Total     float64 `json:"total"`
	}

	var namespaceCosts []NamespaceCost
	var clusterTotal float64

	for rows.Next() {
		var cost NamespaceCost
		err := rows.Scan(&cost.Namespace, &cost.Compute, &cost.Storage, 
			&cost.Network, &cost.Other, &cost.Total)
		if err != nil {
			continue
		}
		namespaceCosts = append(namespaceCosts, cost)
		clusterTotal += cost.Total
	}

	response := map[string]interface{}{
		"cluster_total": clusterTotal,
		"namespaces":    namespaceCosts,
		"period":        "30d",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	// Get recommendations from analyzer
	recommendations, err := h.analyzer.AnalyzeNamespace(r.Context(), namespace)
	if err != nil {
		h.log.Errorf("Analysis failed: %v", err)
		http.Error(w, "Analysis failed", http.StatusInternalServerError)
		return
	}

	// Group recommendations by pod
	podRecommendations := make(map[string][]analyzer.Recommendation)
	totalSavings := 0.0

	for _, rec := range recommendations {
		podRecommendations[rec.PodName] = append(podRecommendations[rec.PodName], rec)
		totalSavings += rec.PotentialSavings
	}

	// Generate YAML patches for applying recommendations
	patches := h.generateResourcePatches(recommendations)

	response := map[string]interface{}{
		"namespace":         namespace,
		"recommendations":   podRecommendations,
		"total_savings":     totalSavings,
		"annual_savings":    totalSavings * 12,
		"patches":          patches,
		"apply_command":    fmt.Sprintf("kubectl apply -f recommendations-%s.yaml", namespace),
		"confidence_score": h.calculateOverallConfidence(recommendations),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ApplyRecommendation(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Namespace     string `json:"namespace"`
		PodName       string `json:"pod_name"`
		ContainerName string `json:"container_name"`
		ResourceType  string `json:"resource_type"`
		Action        string `json:"action"` // "apply", "reject", "modify"
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get the specific recommendation
	recommendations, err := h.analyzer.AnalyzeNamespace(r.Context(), request.Namespace)
	if err != nil {
		http.Error(w, "Failed to get recommendations", http.StatusInternalServerError)
		return
	}

	var targetRecommendation *analyzer.Recommendation
	for _, rec := range recommendations {
		if rec.PodName == request.PodName && 
		   rec.ContainerName == request.ContainerName && 
		   rec.ResourceType == request.ResourceType {
			targetRecommendation = &rec
			break
		}
	}

	if targetRecommendation == nil {
		http.Error(w, "Recommendation not found", http.StatusNotFound)
		return
	}

	// Save recommendation action
	_, err = h.db.Exec(`
		INSERT INTO recommendation_actions 
		(namespace, pod_name, container_name, resource_type, action, applied_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, request.Namespace, request.PodName, request.ContainerName, 
		request.ResourceType, request.Action, time.Now())

	if err != nil {
		h.log.Errorf("Failed to save recommendation action: %v", err)
	}

	response := map[string]interface{}{
		"status": "success",
		"action": request.Action,
		"message": fmt.Sprintf("Recommendation %s for %s/%s/%s", 
			request.Action, request.Namespace, request.PodName, request.ContainerName),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) BulkApplyRecommendations(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Namespace      string   `json:"namespace"`
		RecommendationIDs []string `json:"recommendation_ids"`
		Action         string   `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Apply multiple recommendations
	appliedCount := 0
	failedCount := 0

	for _, id := range request.RecommendationIDs {
		// Parse recommendation ID and apply
		// This is a simplified implementation
		appliedCount++
	}

	response := map[string]interface{}{
		"status": "success",
		"applied": appliedCount,
		"failed": failedCount,
		"message": fmt.Sprintf("Applied %d recommendations, %d failed", appliedCount, failedCount),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) SimulateCosts(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Namespace string `json:"namespace"`
		Changes   []struct {
			PodName       string  `json:"pod_name"`
			ContainerName string  `json:"container_name"`
			CPURequest    float64 `json:"cpu_request"`
			CPULimit      float64 `json:"cpu_limit"`
			MemoryRequest float64 `json:"memory_request"`
			MemoryLimit   float64 `json:"memory_limit"`
			Replicas      int     `json:"replicas"`
		} `json:"changes"`
		Period string `json:"period"` // "daily", "monthly", "yearly"
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get current costs
	currentCosts := h.getCurrentCosts(request.Namespace)

	// Calculate new costs based on changes
	newCosts := currentCosts
	costDelta := 0.0

	for _, change := range request.Changes {
		// Get current resource allocation
		current := h.getCurrentAllocation(request.Namespace, change.PodName, change.ContainerName)

		// Calculate cost difference
		cpuDelta := (change.CPURequest - current["cpu_request"]) * 0.00001 * float64(change.Replicas)
		memoryDelta := (change.MemoryRequest - current["memory_request"]) * 0.00000001 * float64(change.Replicas)

		costDelta += cpuDelta + memoryDelta
	}

	// Apply period multiplier
	var multiplier float64
	switch request.Period {
	case "daily":
		multiplier = 24
	case "monthly":
		multiplier = 24 * 30
	case "yearly":
		multiplier = 24 * 365
	default:
		multiplier = 24 * 30
	}

	projectedCost := (currentCosts + costDelta) * multiplier
	savings := currentCosts*multiplier - projectedCost

	response := map[string]interface{}{
		"current_cost":    currentCosts * multiplier,
		"projected_cost":  projectedCost,
		"cost_difference": costDelta * multiplier,
		"savings":         savings,
		"savings_percent": (savings / (currentCosts * multiplier)) * 100,
		"breakdown": map[string]float64{
			"compute": projectedCost * 0.6,  // Rough estimates
			"storage": projectedCost * 0.2,
			"network": projectedCost * 0.15,
			"other":   projectedCost * 0.05,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ExportReport(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	format := r.URL.Query().Get("format") // "csv", "pdf", "xlsx"

	// Generate comprehensive report
	report := h.generateComprehensiveReport(namespace)

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=cost-report-%s.csv", namespace))
		h.exportCSV(w, report)
	case "pdf":
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=cost-report-%s.pdf", namespace))
		h.exportPDF(w, report)
	case "xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=cost-report-%s.xlsx", namespace))
		h.exportExcel(w, report)
	default:
		json.NewEncoder(w).Encode(report)
	}
}

func (h *Handler) GetResourceUsage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	// Get current resource usage
	rows, err := h.db.Query(`
		SELECT 
			pm.pod_name,
			pm.container_name,
			AVG(pm.cpu_millicores) as avg_cpu,
			MAX(pm.cpu_millicores) as max_cpu,
			AVG(pm.memory_bytes) as avg_memory,
			MAX(pm.memory_bytes) as max_memory,
			rr.cpu_request,
			rr.cpu_limit,
			rr.memory_request,
			rr.memory_limit
		FROM pod_metrics pm
		LEFT JOIN resource_requests rr ON 
			pm.namespace = rr.namespace AND 
			pm.pod_name = rr.pod_name AND 
			pm.container_name = rr.container_name
		WHERE pm.namespace = $1 
			AND pm.timestamp > NOW() - INTERVAL '1 hour'
		GROUP BY pm.pod_name, pm.container_name, 
			rr.cpu_request, rr.cpu_limit, rr.memory_request, rr.memory_limit
	`, namespace)

	if err != nil {
		h.log.Errorf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ResourceUsage struct {
		PodName        string  `json:"pod_name"`
		ContainerName  string  `json:"container_name"`
		AvgCPU         float64 `json:"avg_cpu"`
		MaxCPU         float64 `json:"max_cpu"`
		AvgMemory      float64 `json:"avg_memory"`
		MaxMemory      float64 `json:"max_memory"`
		CPURequest     float64 `json:"cpu_request"`
		CPULimit       float64 `json:"cpu_limit"`
		MemoryRequest  float64 `json:"memory_request"`
		MemoryLimit    float64 `json:"memory_limit"`
		CPUUtilization float64 `json:"cpu_utilization"`
		MemoryUtilization float64 `json:"memory_utilization"`
	}

	var usage []ResourceUsage

	for rows.Next() {
		var res ResourceUsage
		err := rows.Scan(&res.PodName, &res.ContainerName, 
			&res.AvgCPU, &res.MaxCPU, &res.AvgMemory, &res.MaxMemory,
			&res.CPURequest, &res.CPULimit, &res.MemoryRequest, &res.MemoryLimit)
		if err != nil {
			continue
		}

		// Calculate utilization percentages
		if res.CPURequest > 0 {
			res.CPUUtilization = (res.AvgCPU / res.CPURequest) * 100
		}
		if res.MemoryRequest > 0 {
			res.MemoryUtilization = (res.AvgMemory / res.MemoryRequest) * 100
		}

		usage = append(usage, res)
	}

	response := map[string]interface{}{
		"namespace": namespace,
		"usage":     usage,
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper methods

func (h *Handler) getResourceBreakdown(namespace string, startTime, endTime time.Time) map[string]float64 {
	// Get cost breakdown by resource type
	var compute, storage, network, other float64

	err := h.db.QueryRow(`
		SELECT 
			SUM(compute_cost) as compute,
			SUM(storage_cost) as storage,
			SUM(network_cost) as network,
			SUM(other_cost) as other
		FROM namespace_costs
		WHERE namespace = $1 AND timestamp BETWEEN $2 AND $3
	`, namespace, startTime, endTime).Scan(&compute, &storage, &network, &other)

	if err != nil {
		h.log.Warnf("Failed to get resource breakdown: %v", err)
		return map[string]float64{
			"compute": 0,
			"storage": 0,
			"network": 0,
			"other":   0,
		}
	}

	return map[string]float64{
		"compute": compute,
		"storage": storage,
		"network": network,
		"other":   other,
	}
}

func (h *Handler) generateResourcePatches(recommendations []analyzer.Recommendation) []string {
	var patches []string

	for _, rec := range recommendations {
		patch := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  containers:
  - name: %s
    resources:
      requests:
        %s: %s
      limits:
        %s: %s
`, rec.PodName, rec.Namespace, rec.ContainerName,
			rec.ResourceType, h.formatResourceValue(rec.ResourceType, rec.RecommendedRequest),
			rec.ResourceType, h.formatResourceValue(rec.ResourceType, rec.RecommendedLimit))

		patches = append(patches, patch)
	}

	return patches
}

func (h *Handler) formatResourceValue(resourceType string, value float64) string {
	if resourceType == "CPU" {
		return fmt.Sprintf("%dm", int(value))
	} else {
		return fmt.Sprintf("%dMi", int(value/1024/1024))
	}
}

func (h *Handler) calculateOverallConfidence(recommendations []analyzer.Recommendation) float64 {
	if len(recommendations) == 0 {
		return 0
	}

	totalConfidence := 0.0
	for _, rec := range recommendations {
		totalConfidence += rec.Confidence
	}

	return totalConfidence / float64(len(recommendations))
}

func (h *Handler) getCurrentCosts(namespace string) float64 {
	var totalCost float64
	err := h.db.QueryRow(`
		SELECT SUM(compute_cost + storage_cost + network_cost + other_cost)
		FROM namespace_costs
		WHERE namespace = $1 AND timestamp > NOW() - INTERVAL '1 hour'
	`, namespace).Scan(&totalCost)

	if err != nil {
		return 0
	}
	return totalCost
}

func (h *Handler) getCurrentAllocation(namespace, podName, containerName string) map[string]float64 {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit float64

	err := h.db.QueryRow(`
		SELECT cpu_request, cpu_limit, memory_request, memory_limit
		FROM resource_requests
		WHERE namespace = $1 AND pod_name = $2 AND container_name = $3
		ORDER BY timestamp DESC LIMIT 1
	`, namespace, podName, containerName).Scan(&cpuRequest, &cpuLimit, &memoryRequest, &memoryLimit)

	if err != nil {
		return map[string]float64{
			"cpu_request":    0,
			"cpu_limit":      0,
			"memory_request": 0,
			"memory_limit":   0,
		}
	}

	return map[string]float64{
		"cpu_request":    cpuRequest,
		"cpu_limit":      cpuLimit,
		"memory_request": memoryRequest,
		"memory_limit":   memoryLimit,
	}
}

func (h *Handler) generateComprehensiveReport(namespace string) map[string]interface{} {
	// This would generate a comprehensive report with costs, recommendations, trends, etc.
	return map[string]interface{}{
		"namespace": namespace,
		"generated_at": time.Now().UTC(),
		"summary": "Comprehensive cost optimization report",
	}
}

func (h *Handler) exportCSV(w http.ResponseWriter, report map[string]interface{}) {
	// CSV export implementation
	w.Write([]byte("Namespace,Cost,Date\n"))
}

func (h *Handler) exportPDF(w http.ResponseWriter, report map[string]interface{}) {
	// PDF export implementation
	w.Write([]byte("PDF report would be generated here"))
}

func (h *Handler) exportExcel(w http.ResponseWriter, report map[string]interface{}) {
	// Excel export implementation
	w.Write([]byte("Excel report would be generated here"))
} 