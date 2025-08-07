package cloudprovider

import (
	"context"
	"time"
)

// Provider interface for cloud cost management
type Provider interface {
	GetNodeCosts(ctx context.Context) (map[string]float64, error)
	GetDetailedCosts(ctx context.Context, start, end time.Time) (*CostBreakdown, error)
	GetClusterCosts(ctx context.Context, clusterName string) (*ClusterCosts, error)
}

// CostBreakdown represents detailed cost information
type CostBreakdown struct {
	Namespaces map[string]NamespaceCost `json:"namespaces"`
	Total      float64                  `json:"total"`
	Period     string                   `json:"period"`
}

// NamespaceCost represents cost breakdown by namespace
type NamespaceCost struct {
	Total   float64 `json:"total"`
	Compute float64 `json:"compute"`
	Storage float64 `json:"storage"`
	Network float64 `json:"network"`
	Other   float64 `json:"other"`
}

// ClusterCosts represents cluster-level cost information
type ClusterCosts struct {
	ClusterName string                  `json:"cluster_name"`
	Total       float64                 `json:"total"`
	Nodes       map[string]NodeCost     `json:"nodes"`
	Namespaces  map[string]NamespaceCost `json:"namespaces"`
	Period      string                  `json:"period"`
}

// NodeCost represents cost information for a single node
type NodeCost struct {
	InstanceType string  `json:"instance_type"`
	Region       string  `json:"region"`
	HourlyCost   float64 `json:"hourly_cost"`
	MonthlyCost  float64 `json:"monthly_cost"`
	Components   struct {
		Compute float64 `json:"compute"`
		Storage float64 `json:"storage"`
		Network float64 `json:"network"`
		Other   float64 `json:"other"`
	} `json:"components"`
}

// MockCostProvider provides mock cost data for testing
type MockCostProvider struct {
	clusterName string
}

func NewMockCostProvider() *MockCostProvider {
	return &MockCostProvider{
		clusterName: "mock-cluster",
	}
}

func (m *MockCostProvider) GetNodeCosts(ctx context.Context) (map[string]float64, error) {
	// Return mock node costs
	return map[string]float64{
		"node-1": 0.50,  // $0.50 per hour
		"node-2": 0.75,  // $0.75 per hour
		"node-3": 1.25,  // $1.25 per hour
	}, nil
}

func (m *MockCostProvider) GetDetailedCosts(ctx context.Context, start, end time.Time) (*CostBreakdown, error) {
	// Return mock detailed costs
	return &CostBreakdown{
		Namespaces: map[string]NamespaceCost{
			"default": {
				Total:   150.0,
				Compute: 100.0,
				Storage: 30.0,
				Network: 15.0,
				Other:   5.0,
			},
			"production": {
				Total:   450.0,
				Compute: 300.0,
				Storage: 90.0,
				Network: 45.0,
				Other:   15.0,
			},
			"staging": {
				Total:   75.0,
				Compute: 50.0,
				Storage: 15.0,
				Network: 7.5,
				Other:   2.5,
			},
		},
		Total:  675.0,
		Period: "30d",
	}, nil
}

func (m *MockCostProvider) GetClusterCosts(ctx context.Context, clusterName string) (*ClusterCosts, error) {
	// Return mock cluster costs
	return &ClusterCosts{
		ClusterName: clusterName,
		Total:       675.0,
		Nodes: map[string]NodeCost{
			"node-1": {
				InstanceType: "t3.medium",
				Region:       "us-west-2",
				HourlyCost:   0.50,
				MonthlyCost:  360.0,
				Components: struct {
					Compute float64 `json:"compute"`
					Storage float64 `json:"storage"`
					Network float64 `json:"network"`
					Other   float64 `json:"other"`
				}{
					Compute: 240.0,
					Storage: 72.0,
					Network: 36.0,
					Other:   12.0,
				},
			},
			"node-2": {
				InstanceType: "t3.large",
				Region:       "us-west-2",
				HourlyCost:   0.75,
				MonthlyCost:  540.0,
				Components: struct {
					Compute float64 `json:"compute"`
					Storage float64 `json:"storage"`
					Network float64 `json:"network"`
					Other   float64 `json:"other"`
				}{
					Compute: 360.0,
					Storage: 108.0,
					Network: 54.0,
					Other:   18.0,
				},
			},
		},
		Namespaces: map[string]NamespaceCost{
			"default": {
				Total:   150.0,
				Compute: 100.0,
				Storage: 30.0,
				Network: 15.0,
				Other:   5.0,
			},
			"production": {
				Total:   450.0,
				Compute: 300.0,
				Storage: 90.0,
				Network: 45.0,
				Other:   15.0,
			},
		},
		Period: "30d",
	}, nil
} 