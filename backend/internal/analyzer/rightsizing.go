package analyzer

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"
)

type RightsizingAnalyzer struct {
	db                *sql.DB
	wasteThreshold    float64  // Default 30%
	analysisWindow    time.Duration
	minDataPoints     int
	confidenceLevel   float64
	log               *logrus.Logger
}

type Recommendation struct {
	Namespace         string
	PodName           string
	ContainerName     string
	ResourceType      string
	CurrentRequest    float64
	CurrentLimit      float64
	RecommendedRequest float64
	RecommendedLimit  float64
	P50Usage          float64
	P95Usage          float64
	P99Usage          float64
	MaxUsage          float64
	PotentialSavings  float64
	Confidence        float64
	Reasoning         string
	RiskLevel         string
	LastUpdated       time.Time
}

type ResourceAllocation struct {
	CPURequest    float64
	CPULimit      float64
	MemoryRequest float64
	MemoryLimit   float64
}

func NewRightsizingAnalyzer(db *sql.DB) *RightsizingAnalyzer {
	return &RightsizingAnalyzer{
		db:              db,
		wasteThreshold:  0.30, // 30% waste threshold
		analysisWindow:  7 * 24 * time.Hour, // 7 days
		minDataPoints:   100,  // Minimum data points for analysis
		confidenceLevel: 0.7,  // 70% confidence threshold
		log:             logrus.New(),
	}
}

func (ra *RightsizingAnalyzer) AnalyzeNamespace(ctx context.Context, namespace string) ([]Recommendation, error) {
	// Query historical metrics for the namespace
	rows, err := ra.db.QueryContext(ctx, `
		SELECT 
			pm.pod_name,
			pm.container_name,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY pm.cpu_millicores) as p50_cpu,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY pm.cpu_millicores) as p95_cpu,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY pm.cpu_millicores) as p99_cpu,
			MAX(pm.cpu_millicores) as max_cpu,
			AVG(pm.cpu_millicores) as avg_cpu,
			STDDEV(pm.cpu_millicores) as stddev_cpu,
			COUNT(*) as data_points,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY pm.memory_bytes) as p50_mem,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY pm.memory_bytes) as p95_mem,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY pm.memory_bytes) as p99_mem,
			MAX(pm.memory_bytes) as max_mem,
			AVG(pm.memory_bytes) as avg_mem,
			STDDEV(pm.memory_bytes) as stddev_mem
		FROM pod_metrics pm
		WHERE 
			pm.namespace = $1 
			AND pm.timestamp > NOW() - INTERVAL '7 days'
		GROUP BY pm.pod_name, pm.container_name
		HAVING COUNT(*) >= $2
	`, namespace, ra.minDataPoints)
	
	if err != nil {
		return nil, fmt.Errorf("querying metrics: %w", err)
	}
	defer rows.Close()

	var recommendations []Recommendation

	for rows.Next() {
		var podName, containerName string
		var p50CPU, p95CPU, p99CPU, maxCPU, avgCPU, stddevCPU float64
		var dataPoints int
		var p50Mem, p95Mem, p99Mem, maxMem, avgMem, stddevMem float64

		err := rows.Scan(&podName, &containerName, 
			&p50CPU, &p95CPU, &p99CPU, &maxCPU, &avgCPU, &stddevCPU, &dataPoints,
			&p50Mem, &p95Mem, &p99Mem, &maxMem, &avgMem, &stddevMem)

		if err != nil {
			ra.log.Warnf("Failed to scan metrics for %s/%s: %v", podName, containerName, err)
			continue
		}

		// Get current resource requests/limits from database
		currentRequests, currentLimits, err := ra.getCurrentResources(namespace, podName, containerName)
		if err != nil {
			ra.log.Warnf("Failed to get current resources for %s/%s: %v", podName, containerName, err)
			continue
		}

		// CPU Recommendation
		cpuRec := ra.calculateCPURecommendation(
			currentRequests.CPURequest, currentLimits.CPULimit,
			p50CPU, p95CPU, p99CPU, maxCPU, avgCPU, stddevCPU,
			dataPoints,
		)

		if cpuRec != nil {
			cpuRec.Namespace = namespace
			cpuRec.PodName = podName
			cpuRec.ContainerName = containerName
			cpuRec.LastUpdated = time.Now()
			recommendations = append(recommendations, *cpuRec)
		}

		// Memory Recommendation
		memRec := ra.calculateMemoryRecommendation(
			currentRequests.MemoryRequest, currentLimits.MemoryLimit,
			p50Mem, p95Mem, p99Mem, maxMem, avgMem, stddevMem,
			dataPoints,
		)

		if memRec != nil {
			memRec.Namespace = namespace
			memRec.PodName = podName
			memRec.ContainerName = containerName
			memRec.LastUpdated = time.Now()
			recommendations = append(recommendations, *memRec)
		}
	}

	return recommendations, nil
}

func (ra *RightsizingAnalyzer) calculateCPURecommendation(
	currentRequest, currentLimit,
	p50, p95, p99, max, avg, stddev float64,
	dataPoints int,
) *Recommendation {
	// Calculate coefficient of variation for stability check
	cv := stddev / avg
	if avg == 0 {
		cv = 0
	}

	// Determine confidence based on data points and variability
	confidence := ra.calculateConfidence(dataPoints, cv)

	// Calculate recommended values
	// Use P95 for request with a safety margin
	safetyMargin := 1.15 // 15% safety margin
	recommendedRequest := p95 * safetyMargin

	// Use P99 or max for limit based on variability
	var recommendedLimit float64
	var reasoning string
	var riskLevel string

	if cv < 0.3 { // Low variability
		recommendedLimit = p99 * 1.2
		reasoning = "Low variability workload, using P99 + 20% for limit"
		riskLevel = "LOW"
	} else if cv < 0.6 { // Medium variability
		recommendedLimit = math.Max(p99*1.5, max)
		reasoning = "Medium variability workload, using max(P99*1.5, max) for limit"
		riskLevel = "MEDIUM"
	} else { // High variability
		recommendedLimit = max * 1.3
		reasoning = "High variability workload, using max + 30% for limit"
		riskLevel = "HIGH"
	}

	// Check if current allocation is wasteful
	waste := (currentRequest - p95) / currentRequest
	if waste < ra.wasteThreshold && confidence > 0.7 {
		return nil // No significant waste
	}

	// Calculate potential savings
	// Assume linear cost model for simplicity
	costPerMillicore := 0.00001 // $0.00001 per millicore per hour
	hourlyCurrentCost := currentRequest * costPerMillicore
	hourlyRecommendedCost := recommendedRequest * costPerMillicore
	monthlySavings := (hourlyCurrentCost - hourlyRecommendedCost) * 24 * 30

	// Ensure recommendations are reasonable
	if recommendedRequest < 10 { // Minimum 10 millicores
		recommendedRequest = 10
		reasoning += " (adjusted to minimum 10m CPU)"
	}

	if recommendedLimit < recommendedRequest*1.5 {
		recommendedLimit = recommendedRequest * 1.5
		reasoning += " (adjusted limit to 1.5x request)"
	}

	return &Recommendation{
		ResourceType:       "CPU",
		CurrentRequest:     currentRequest,
		CurrentLimit:       currentLimit,
		RecommendedRequest: recommendedRequest,
		RecommendedLimit:   recommendedLimit,
		P50Usage:          p50,
		P95Usage:          p95,
		P99Usage:          p99,
		MaxUsage:          max,
		PotentialSavings:  monthlySavings,
		Confidence:        confidence,
		Reasoning:         reasoning,
		RiskLevel:         riskLevel,
	}
}

func (ra *RightsizingAnalyzer) calculateMemoryRecommendation(
	currentRequest, currentLimit,
	p50, p95, p99, max, avg, stddev float64,
	dataPoints int,
) *Recommendation {
	// Memory recommendations are more conservative due to OOM risks
	cv := stddev / avg
	if avg == 0 {
		cv = 0
	}
	
	confidence := ra.calculateConfidence(dataPoints, cv)

	// For memory, always use max observed + buffer to avoid OOM
	oomBuffer := 1.2 // 20% buffer
	recommendedRequest := p95 * 1.1
	recommendedLimit := max * oomBuffer

	// Round to nearest sensible value (Mi)
	recommendedRequest = math.Ceil(recommendedRequest/1048576) * 1048576
	recommendedLimit = math.Ceil(recommendedLimit/1048576) * 1048576

	waste := (currentRequest - p95) / currentRequest
	if waste < ra.wasteThreshold && confidence > 0.7 {
		return nil
	}

	// Calculate savings (memory typically more expensive than CPU)
	costPerByte := 0.00000001 // $0.00000001 per byte per hour
	hourlyCurrentCost := currentRequest * costPerByte
	hourlyRecommendedCost := recommendedRequest * costPerByte
	monthlySavings := (hourlyCurrentCost - hourlyRecommendedCost) * 24 * 30

	// Determine risk level based on variability
	var riskLevel string
	if cv < 0.3 {
		riskLevel = "LOW"
	} else if cv < 0.6 {
		riskLevel = "MEDIUM"
	} else {
		riskLevel = "HIGH"
	}

	// Ensure minimum memory allocation
	if recommendedRequest < 64*1024*1024 { // 64 Mi minimum
		recommendedRequest = 64 * 1024 * 1024
	}

	if recommendedLimit < recommendedRequest*1.5 {
		recommendedLimit = recommendedRequest * 1.5
	}

	return &Recommendation{
		ResourceType:       "Memory",
		CurrentRequest:     currentRequest,
		CurrentLimit:       currentLimit,
		RecommendedRequest: recommendedRequest,
		RecommendedLimit:   recommendedLimit,
		P50Usage:          p50,
		P95Usage:          p95,
		P99Usage:          p99,
		MaxUsage:          max,
		PotentialSavings:  monthlySavings,
		Confidence:        confidence,
		Reasoning:         "Memory recommendation with OOM prevention buffer",
		RiskLevel:         riskLevel,
	}
}

func (ra *RightsizingAnalyzer) calculateConfidence(dataPoints int, cv float64) float64 {
	// Base confidence on data points
	baseConfidence := math.Min(float64(dataPoints)/1000.0, 1.0) // Max 1000 data points

	// Adjust for variability
	variabilityFactor := 1.0 - (cv * 0.5) // Higher CV reduces confidence
	if variabilityFactor < 0.3 {
		variabilityFactor = 0.3 // Minimum 30% confidence
	}

	// Combine factors
	confidence := baseConfidence * variabilityFactor

	// Ensure reasonable bounds
	if confidence < 0.1 {
		confidence = 0.1
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

func (ra *RightsizingAnalyzer) getCurrentResources(namespace, podName, containerName string) (*ResourceAllocation, *ResourceAllocation, error) {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit float64

	err := ra.db.QueryRow(`
		SELECT cpu_request, cpu_limit, memory_request, memory_limit 
		FROM resource_requests 
		WHERE namespace = $1 AND pod_name = $2 AND container_name = $3
		ORDER BY timestamp DESC LIMIT 1
	`, namespace, podName, containerName).Scan(&cpuRequest, &cpuLimit, &memoryRequest, &memoryLimit)

	if err != nil {
		return nil, nil, fmt.Errorf("getting current resources: %w", err)
	}

	requests := &ResourceAllocation{
		CPURequest:    cpuRequest,
		CPULimit:      cpuLimit,
		MemoryRequest: memoryRequest,
		MemoryLimit:   memoryLimit,
	}

	limits := &ResourceAllocation{
		CPURequest:    cpuLimit,
		CPULimit:      cpuLimit,
		MemoryRequest: memoryLimit,
		MemoryLimit:   memoryLimit,
	}

	return requests, limits, nil
}

func (ra *RightsizingAnalyzer) GetRecommendationHistory(ctx context.Context, namespace string) ([]Recommendation, error) {
	rows, err := ra.db.QueryContext(ctx, `
		SELECT 
			namespace, pod_name, container_name, resource_type,
			current_request, current_limit, recommended_request, recommended_limit,
			p50_usage, p95_usage, p99_usage, max_usage,
			potential_savings, confidence, reasoning, risk_level, created_at
		FROM recommendations
		WHERE namespace = $1
		ORDER BY created_at DESC
	`, namespace)

	if err != nil {
		return nil, fmt.Errorf("querying recommendation history: %w", err)
	}
	defer rows.Close()

	var recommendations []Recommendation

	for rows.Next() {
		var rec Recommendation
		var createdAt time.Time

		err := rows.Scan(
			&rec.Namespace, &rec.PodName, &rec.ContainerName, &rec.ResourceType,
			&rec.CurrentRequest, &rec.CurrentLimit, &rec.RecommendedRequest, &rec.RecommendedLimit,
			&rec.P50Usage, &rec.P95Usage, &rec.P99Usage, &rec.MaxUsage,
			&rec.PotentialSavings, &rec.Confidence, &rec.Reasoning, &rec.RiskLevel, &createdAt,
		)

		if err != nil {
			ra.log.Warnf("Failed to scan recommendation: %v", err)
			continue
		}

		rec.LastUpdated = createdAt
		recommendations = append(recommendations, rec)
	}

	return recommendations, nil
}

func (ra *RightsizingAnalyzer) SaveRecommendation(ctx context.Context, rec *Recommendation) error {
	_, err := ra.db.ExecContext(ctx, `
		INSERT INTO recommendations 
		(namespace, pod_name, container_name, resource_type,
		 current_request, current_limit, recommended_request, recommended_limit,
		 p50_usage, p95_usage, p99_usage, max_usage,
		 potential_savings, confidence, reasoning, risk_level, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, rec.Namespace, rec.PodName, rec.ContainerName, rec.ResourceType,
		rec.CurrentRequest, rec.CurrentLimit, rec.RecommendedRequest, rec.RecommendedLimit,
		rec.P50Usage, rec.P95Usage, rec.P99Usage, rec.MaxUsage,
		rec.PotentialSavings, rec.Confidence, rec.Reasoning, rec.RiskLevel, rec.LastUpdated)

	return err
}

func (ra *RightsizingAnalyzer) GetOptimizationSummary(ctx context.Context, namespace string) (map[string]interface{}, error) {
	recommendations, err := ra.AnalyzeNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var totalSavings float64
	var cpuSavings, memorySavings float64
	var highConfidenceCount, mediumConfidenceCount, lowConfidenceCount int
	var highRiskCount, mediumRiskCount, lowRiskCount int

	for _, rec := range recommendations {
		totalSavings += rec.PotentialSavings

		if rec.ResourceType == "CPU" {
			cpuSavings += rec.PotentialSavings
		} else {
			memorySavings += rec.PotentialSavings
		}

		// Count by confidence
		if rec.Confidence >= 0.8 {
			highConfidenceCount++
		} else if rec.Confidence >= 0.6 {
			mediumConfidenceCount++
		} else {
			lowConfidenceCount++
		}

		// Count by risk
		switch rec.RiskLevel {
		case "LOW":
			lowRiskCount++
		case "MEDIUM":
			mediumRiskCount++
		case "HIGH":
			highRiskCount++
		}
	}

	return map[string]interface{}{
		"total_recommendations": len(recommendations),
		"total_savings":         totalSavings,
		"annual_savings":        totalSavings * 12,
		"cpu_savings":           cpuSavings,
		"memory_savings":        memorySavings,
		"confidence_breakdown": map[string]int{
			"high":   highConfidenceCount,
			"medium": mediumConfidenceCount,
			"low":    lowConfidenceCount,
		},
		"risk_breakdown": map[string]int{
			"low":    lowRiskCount,
			"medium": mediumRiskCount,
			"high":   highRiskCount,
		},
		"optimization_potential": (totalSavings / 1000) * 100, // Percentage of $1000 baseline
	}, nil
} 