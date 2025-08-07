-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Namespace metrics table
CREATE TABLE IF NOT EXISTS namespace_metrics (
    namespace VARCHAR(255) NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (namespace, metric_type, timestamp)
);

-- Convert to hypertable for time-series optimization
SELECT create_hypertable('namespace_metrics', 'timestamp', if_not_exists => TRUE);

-- Pod metrics table
CREATE TABLE IF NOT EXISTS pod_metrics (
    namespace VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    container_name VARCHAR(255) NOT NULL,
    cpu_millicores DOUBLE PRECISION,
    memory_bytes DOUBLE PRECISION,
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (namespace, pod_name, container_name, timestamp)
);

SELECT create_hypertable('pod_metrics', 'timestamp', if_not_exists => TRUE);

-- Node metrics table
CREATE TABLE IF NOT EXISTS node_metrics (
    node_name VARCHAR(255) NOT NULL,
    cpu_millicores DOUBLE PRECISION,
    memory_bytes DOUBLE PRECISION,
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (node_name, timestamp)
);

SELECT create_hypertable('node_metrics', 'timestamp', if_not_exists => TRUE);

-- Storage metrics table
CREATE TABLE IF NOT EXISTS storage_metrics (
    namespace VARCHAR(255) NOT NULL,
    pvc_name VARCHAR(255) NOT NULL,
    used_bytes DOUBLE PRECISION,
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (namespace, pvc_name, timestamp)
);

SELECT create_hypertable('storage_metrics', 'timestamp', if_not_exists => TRUE);

-- Resource requests table
CREATE TABLE IF NOT EXISTS resource_requests (
    namespace VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    container_name VARCHAR(255) NOT NULL,
    cpu_request DOUBLE PRECISION,
    cpu_limit DOUBLE PRECISION,
    memory_request DOUBLE PRECISION,
    memory_limit DOUBLE PRECISION,
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (namespace, pod_name, container_name, timestamp)
);

SELECT create_hypertable('resource_requests', 'timestamp', if_not_exists => TRUE);

-- Namespace costs table
CREATE TABLE IF NOT EXISTS namespace_costs (
    namespace VARCHAR(255) NOT NULL,
    compute_cost DECIMAL(10, 4),
    storage_cost DECIMAL(10, 4),
    network_cost DECIMAL(10, 4),
    other_cost DECIMAL(10, 4),
    timestamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (namespace, timestamp)
);

SELECT create_hypertable('namespace_costs', 'timestamp', if_not_exists => TRUE);

-- Recommendations history
CREATE TABLE IF NOT EXISTS recommendations (
    id SERIAL PRIMARY KEY,
    namespace VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    container_name VARCHAR(255) NOT NULL,
    resource_type VARCHAR(20) NOT NULL,
    current_request DOUBLE PRECISION,
    current_limit DOUBLE PRECISION,
    recommended_request DOUBLE PRECISION,
    recommended_limit DOUBLE PRECISION,
    p50_usage DOUBLE PRECISION,
    p95_usage DOUBLE PRECISION,
    p99_usage DOUBLE PRECISION,
    max_usage DOUBLE PRECISION,
    potential_savings DECIMAL(10, 4),
    confidence DOUBLE PRECISION,
    reasoning TEXT,
    risk_level VARCHAR(20),
    applied BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    applied_at TIMESTAMPTZ
);

-- Recommendation actions table
CREATE TABLE IF NOT EXISTS recommendation_actions (
    id SERIAL PRIMARY KEY,
    namespace VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    container_name VARCHAR(255) NOT NULL,
    resource_type VARCHAR(20) NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'apply', 'reject', 'modify'
    applied_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_namespace_metrics_namespace ON namespace_metrics(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_pod_metrics_namespace ON pod_metrics(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_node_metrics_node ON node_metrics(node_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_storage_metrics_namespace ON storage_metrics(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_resource_requests_namespace ON resource_requests(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_namespace_costs_namespace ON namespace_costs(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_recommendations_namespace ON recommendations(namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recommendation_actions_namespace ON recommendation_actions(namespace, applied_at DESC);

-- Retention policy (keep 90 days of detailed data)
SELECT add_retention_policy('namespace_metrics', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('pod_metrics', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('node_metrics', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('storage_metrics', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('resource_requests', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('namespace_costs', INTERVAL '90 days', if_not_exists => TRUE);

-- Continuous aggregates for faster queries
CREATE MATERIALIZED VIEW IF NOT EXISTS hourly_namespace_metrics
WITH (timescaledb.continuous) AS
SELECT
    namespace,
    metric_type,
    time_bucket('1 hour', timestamp) AS hour,
    AVG(value) as avg_value,
    MAX(value) as max_value,
    MIN(value) as min_value,
    COUNT(*) as data_points
FROM namespace_metrics
GROUP BY namespace, metric_type, hour;

SELECT add_continuous_aggregate_policy('hourly_namespace_metrics',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- Daily cost aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS daily_namespace_costs
WITH (timescaledb.continuous) AS
SELECT
    namespace,
    time_bucket('1 day', timestamp) AS day,
    SUM(compute_cost) as total_compute,
    SUM(storage_cost) as total_storage,
    SUM(network_cost) as total_network,
    SUM(other_cost) as total_other,
    SUM(compute_cost + storage_cost + network_cost + other_cost) as total_cost
FROM namespace_costs
GROUP BY namespace, day;

SELECT add_continuous_aggregate_policy('daily_namespace_costs',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

-- Pod utilization aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS hourly_pod_utilization
WITH (timescaledb.continuous) AS
SELECT
    namespace,
    pod_name,
    container_name,
    time_bucket('1 hour', timestamp) AS hour,
    AVG(cpu_millicores) as avg_cpu,
    MAX(cpu_millicores) as max_cpu,
    AVG(memory_bytes) as avg_memory,
    MAX(memory_bytes) as max_memory
FROM pod_metrics
GROUP BY namespace, pod_name, container_name, hour;

SELECT add_continuous_aggregate_policy('hourly_pod_utilization',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- Create views for easier querying
CREATE OR REPLACE VIEW current_resource_usage AS
SELECT 
    pm.namespace,
    pm.pod_name,
    pm.container_name,
    AVG(pm.cpu_millicores) as avg_cpu,
    MAX(pm.cpu_millicores) as max_cpu,
    AVG(pm.memory_bytes) as avg_memory,
    MAX(pm.memory_bytes) as max_memory,
    rr.cpu_request,
    rr.cpu_limit,
    rr.memory_request,
    rr.memory_limit,
    CASE 
        WHEN rr.cpu_request > 0 THEN (AVG(pm.cpu_millicores) / rr.cpu_request) * 100
        ELSE 0 
    END as cpu_utilization_percent,
    CASE 
        WHEN rr.memory_request > 0 THEN (AVG(pm.memory_bytes) / rr.memory_request) * 100
        ELSE 0 
    END as memory_utilization_percent
FROM pod_metrics pm
LEFT JOIN resource_requests rr ON 
    pm.namespace = rr.namespace AND 
    pm.pod_name = rr.pod_name AND 
    pm.container_name = rr.container_name
WHERE pm.timestamp > NOW() - INTERVAL '1 hour'
GROUP BY pm.namespace, pm.pod_name, pm.container_name,
    rr.cpu_request, rr.cpu_limit, rr.memory_request, rr.memory_limit;

-- Create view for cost optimization opportunities
CREATE OR REPLACE VIEW optimization_opportunities AS
SELECT 
    namespace,
    pod_name,
    container_name,
    'CPU' as resource_type,
    cpu_request as current_request,
    cpu_limit as current_limit,
    CASE 
        WHEN cpu_utilization_percent < 30 THEN cpu_request * 0.7
        WHEN cpu_utilization_percent < 50 THEN cpu_request * 0.8
        ELSE cpu_request
    END as recommended_request,
    CASE 
        WHEN cpu_utilization_percent < 30 THEN cpu_limit * 0.8
        WHEN cpu_utilization_percent < 50 THEN cpu_limit * 0.9
        ELSE cpu_limit
    END as recommended_limit,
    cpu_utilization_percent as utilization_percent,
    CASE 
        WHEN cpu_utilization_percent < 30 THEN 'HIGH'
        WHEN cpu_utilization_percent < 50 THEN 'MEDIUM'
        ELSE 'LOW'
    END as optimization_potential
FROM current_resource_usage
WHERE cpu_utilization_percent < 70

UNION ALL

SELECT 
    namespace,
    pod_name,
    container_name,
    'Memory' as resource_type,
    memory_request as current_request,
    memory_limit as current_limit,
    CASE 
        WHEN memory_utilization_percent < 30 THEN memory_request * 0.7
        WHEN memory_utilization_percent < 50 THEN memory_request * 0.8
        ELSE memory_request
    END as recommended_request,
    CASE 
        WHEN memory_utilization_percent < 30 THEN memory_limit * 0.8
        WHEN memory_utilization_percent < 50 THEN memory_limit * 0.9
        ELSE memory_limit
    END as recommended_limit,
    memory_utilization_percent as utilization_percent,
    CASE 
        WHEN memory_utilization_percent < 30 THEN 'HIGH'
        WHEN memory_utilization_percent < 50 THEN 'MEDIUM'
        ELSE 'LOW'
    END as optimization_potential
FROM current_resource_usage
WHERE memory_utilization_percent < 70; 