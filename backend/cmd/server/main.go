package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s-cost-optimizer/internal/api"
	"k8s-cost-optimizer/internal/analyzer"
	"k8s-cost-optimizer/internal/collectors"
	"k8s-cost-optimizer/internal/database"
	"k8s-cost-optimizer/pkg/cloudprovider"
	"k8s-cost-optimizer/pkg/kubernetes"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	_ "github.com/lib/pq"
)

var log = logrus.New()

func main() {
	// Initialize configuration
	initConfig()

	// Initialize logger
	initLogger()

	log.Info("Starting Kubernetes Cost Optimizer...")

	// Initialize database connection
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Redis cache
	redisClient, err := initRedis()
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize Kubernetes client
	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Initialize cloud provider
	costProvider, err := initCloudProvider()
	if err != nil {
		log.Fatalf("Failed to initialize cloud provider: %v", err)
	}

	// Initialize components
	metricsCollector := collectors.NewMetricsCollector(k8sClient, db)
	rightsizingAnalyzer := analyzer.NewRightsizingAnalyzer(db)
	handler := api.NewHandler(rightsizingAnalyzer, metricsCollector, costProvider, db, redisClient)

	// Initialize router
	router := initRouter(handler)

	// Start metrics collection in background
	go startMetricsCollection(metricsCollector)

	// Start cost collection in background
	go startCostCollection(metricsCollector, costProvider)

	// Start server
	server := &http.Server{
		Addr:         viper.GetString("server.port"),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Server shutdown error: %v", err)
		}
	}()

	log.Infof("Server starting on port %s", viper.GetString("server.port"))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/k8s-cost-optimizer")

	// Set defaults
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.name", "k8s_cost_optimizer")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("prometheus.url", "http://prometheus:9090")
	viper.SetDefault("metrics.collection_interval", "5m")
	viper.SetDefault("cost.collection_interval", "1h")

	// Read environment variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Warnf("Config file not found, using defaults: %v", err)
		}
	}
}

func initLogger() {
	level, err := logrus.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	if viper.GetString("log.format") == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

func initDatabase() (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		viper.GetString("database.host"),
		viper.GetInt("database.port"),
		viper.GetString("database.user"),
		viper.GetString("database.password"),
		viper.GetString("database.name"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Info("Database connection established")
	return db, nil
}

func initRedis() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port")),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Info("Redis connection established")
	return client, nil
}

func initCloudProvider() (cloudprovider.Provider, error) {
	provider := viper.GetString("cloud.provider")
	region := viper.GetString("cloud.region")
	clusterName := viper.GetString("cloud.cluster_name")

	switch provider {
	case "aws":
		return cloudprovider.NewAWSCostProvider(region, clusterName)
	case "azure":
		return cloudprovider.NewAzureCostProvider(region, clusterName)
	case "gcp":
		return cloudprovider.NewGCPCostProvider(region, clusterName)
	default:
		return cloudprovider.NewMockCostProvider(), nil
	}
}

func initRouter(handler *api.Handler) *mux.Router {
	router := mux.NewRouter()

	// Health checks
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	router.HandleFunc("/ready", handler.ReadyCheck).Methods("GET")

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	
	// Cost endpoints
	apiRouter.HandleFunc("/costs/namespace/{namespace}", handler.GetNamespaceCosts).Methods("GET")
	apiRouter.HandleFunc("/costs/cluster", handler.GetClusterCosts).Methods("GET")
	apiRouter.HandleFunc("/costs/simulate", handler.SimulateCosts).Methods("POST")

	// Recommendations endpoints
	apiRouter.HandleFunc("/recommendations/{namespace}", handler.GetRecommendations).Methods("GET")
	apiRouter.HandleFunc("/recommendations/apply", handler.ApplyRecommendation).Methods("POST")
	apiRouter.HandleFunc("/recommendations/bulk-apply", handler.BulkApplyRecommendations).Methods("POST")

	// Export endpoints
	apiRouter.HandleFunc("/export", handler.ExportReport).Methods("GET")

	// Resource endpoints
	apiRouter.HandleFunc("/resources/{namespace}", handler.GetResourceUsage).Methods("GET")
	apiRouter.HandleFunc("/resources/pods/{namespace}", handler.GetPodResources).Methods("GET")

	// Analytics endpoints
	apiRouter.HandleFunc("/analytics/trends/{namespace}", handler.GetCostTrends).Methods("GET")
	apiRouter.HandleFunc("/analytics/anomalies", handler.GetAnomalies).Methods("GET")

	// Middleware
	router.Use(api.LoggingMiddleware)
	router.Use(api.CorsMiddleware)
	router.Use(api.RecoveryMiddleware)

	return router
}

func startMetricsCollection(collector *collectors.MetricsCollector) {
	interval := viper.GetDuration("metrics.collection_interval")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("Starting metrics collection with interval: %v", interval)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			
			if err := collector.CollectNamespaceMetrics(ctx); err != nil {
				log.Errorf("Failed to collect namespace metrics: %v", err)
			}
			
			if err := collector.CollectPodMetrics(ctx); err != nil {
				log.Errorf("Failed to collect pod metrics: %v", err)
			}
			
			cancel()
		}
	}
}

func startCostCollection(collector *collectors.MetricsCollector, costProvider cloudprovider.Provider) {
	interval := viper.GetDuration("cost.collection_interval")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("Starting cost collection with interval: %v", interval)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			
			if err := collector.CollectCosts(ctx, costProvider); err != nil {
				log.Errorf("Failed to collect costs: %v", err)
			}
			
			cancel()
		}
	}
} 