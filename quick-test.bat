@echo off
setlocal enabledelayedexpansion

REM Kubernetes Cost Optimizer - Quick Test Script (Windows)
REM This script provides a fast way to test the application locally

set "SCRIPT_DIR=%~dp0"
set "PROJECT_NAME=k8s-cost-optimizer"
set "DOCKER_COMPOSE_FILE=docker-compose.local.yml"

REM Colors for output (Windows 10+)
set "RED=[91m"
set "GREEN=[92m"
set "YELLOW=[93m"
set "BLUE=[94m"
set "PURPLE=[95m"
set "CYAN=[96m"
set "NC=[0m"

REM Function to print colored output
:print_status
echo %BLUE%[INFO]%NC% %~1
goto :eof

:print_success
echo %GREEN%[SUCCESS]%NC% %~1
goto :eof

:print_warning
echo %YELLOW%[WARNING]%NC% %~1
goto :eof

:print_error
echo %RED%[ERROR]%NC% %~1
goto :eof

:print_header
echo %PURPLE%================================%NC%
echo %PURPLE% %~1%NC%
echo %PURPLE%================================%NC%
goto :eof

REM Function to check prerequisites
:check_prerequisites
call :print_header "Checking Prerequisites"

REM Check if Docker is running
docker info >nul 2>&1
if errorlevel 1 (
    call :print_error "Docker is not running. Please start Docker Desktop."
    exit /b 1
)
call :print_success "Docker is running"

REM Check if Docker Compose is available
docker-compose --version >nul 2>&1
if errorlevel 1 (
    call :print_error "Docker Compose is not installed."
    exit /b 1
)
call :print_success "Docker Compose is available"

REM Check if Make is available
make --version >nul 2>&1
if errorlevel 1 (
    call :print_warning "Make is not available. Some commands may not work."
) else (
    call :print_success "Make is available"
)

REM Check if ports are available
for %%p in (3000 3001 8080 9090 5432 6379) do (
    netstat -an | find "%%p" >nul 2>&1
    if errorlevel 1 (
        call :print_success "Port %%p is available"
    ) else (
        call :print_warning "Port %%p is already in use"
    )
)
goto :eof

REM Function to start the application
:start_application
call :print_header "Starting Kubernetes Cost Optimizer"

REM Navigate to project directory
cd /d "%SCRIPT_DIR%"

REM Create .env file if it doesn't exist
if not exist .env (
    call :print_status "Creating .env file from template..."
    copy env.example .env >nul
    call :print_success "Created .env file"
)

REM Start infrastructure services
call :print_status "Starting infrastructure services..."
docker-compose -f %DOCKER_COMPOSE_FILE% up -d postgres redis prometheus grafana

REM Wait for services to be ready
call :print_status "Waiting for services to be ready..."
timeout /t 10 /nobreak >nul

REM Setup database
call :print_status "Setting up database..."
make --version >nul 2>&1
if not errorlevel 1 (
    make setup-db-local
) else (
    REM Manual database setup
    docker exec k8s-kost-postgres-1 pg_isready -U k8s_cost_user -d k8s_cost_optimizer
    if errorlevel 1 (
        call :print_error "Database is not ready. Retrying..."
        timeout /t 5 /nobreak >nul
        docker exec k8s-kost-postgres-1 pg_isready -U k8s_cost_user -d k8s_cost_optimizer
    )
    
    REM Run migrations
    docker exec -i k8s-kost-postgres-1 psql -U k8s_cost_user -d k8s_cost_optimizer < backend\internal\database\migrations.sql
)
call :print_success "Database setup complete"

REM Generate sample data
call :print_status "Generating sample data..."
make --version >nul 2>&1
if not errorlevel 1 (
    make sample-data
) else (
    REM Manual sample data generation
    call scripts\generate-sample-data.sh
)
call :print_success "Sample data generated"

REM Start application services
call :print_status "Starting application services..."
docker-compose -f %DOCKER_COMPOSE_FILE% up -d backend frontend

call :print_success "Application started successfully!"
goto :eof

REM Function to check application status
:check_status
call :print_header "Application Status"

cd /d "%SCRIPT_DIR%"

REM Check if containers are running
call :print_status "Checking container status..."
docker-compose -f %DOCKER_COMPOSE_FILE% ps

REM Check application health
call :print_status "Checking application health..."

REM Check backend health
curl -f http://localhost:8080/health >nul 2>&1
if errorlevel 1 (
    call :print_error "Backend is not responding"
) else (
    call :print_success "Backend is healthy"
)

REM Check frontend
curl -f http://localhost:3000 >nul 2>&1
if errorlevel 1 (
    call :print_error "Frontend is not responding"
) else (
    call :print_success "Frontend is accessible"
)

REM Check Grafana
curl -f http://localhost:3001 >nul 2>&1
if errorlevel 1 (
    call :print_error "Grafana is not responding"
) else (
    call :print_success "Grafana is accessible"
)

REM Check Prometheus
curl -f http://localhost:9090 >nul 2>&1
if errorlevel 1 (
    call :print_error "Prometheus is not responding"
) else (
    call :print_success "Prometheus is accessible"
)
goto :eof

REM Function to show access information
:show_access_info
call :print_header "Access Information"

echo %CYAN%Application URLs:%NC%
echo   📊 %GREEN%Frontend Dashboard:%NC% http://localhost:3000
echo   🔌 %GREEN%Backend API:%NC% http://localhost:8080
echo   📈 %GREEN%Grafana:%NC% http://localhost:3001 (admin/admin)
echo   📊 %GREEN%Prometheus:%NC% http://localhost:9090

echo.
echo %CYAN%API Endpoints:%NC%
echo   🔍 %GREEN%Health Check:%NC% http://localhost:8080/health
echo   💰 %GREEN%Cost Data:%NC% http://localhost:8080/api/costs/namespace/default
echo   💡 %GREEN%Recommendations:%NC% http://localhost:8080/api/recommendations/default
echo   📊 %GREEN%Metrics:%NC% http://localhost:8080/metrics

echo.
echo %CYAN%Database Access:%NC%
echo   🗄️ %GREEN%PostgreSQL:%NC% localhost:5432
echo   ⚡ %GREEN%Redis:%NC% localhost:6379

echo.
echo %CYAN%Sample Data:%NC%
echo   📊 %GREEN%Namespaces:%NC% default, production, staging, development
echo   🏷️ %GREEN%Time Range:%NC% Last 30 days of mock data
echo   💰 %GREEN%Cost Range:%NC% $100 - $5000 per namespace
goto :eof

REM Function to run quick tests
:run_tests
call :print_header "Running Quick Tests"

REM Test backend health
call :print_status "Testing backend health..."
curl -f http://localhost:8080/health >nul 2>&1
if errorlevel 1 (
    call :print_error "Backend health check failed"
    exit /b 1
)
call :print_success "Backend health check passed"

REM Test cost API
call :print_status "Testing cost API..."
curl -f http://localhost:8080/api/costs/namespace/default >nul 2>&1
if errorlevel 1 (
    call :print_error "Cost API test failed"
    exit /b 1
)
call :print_success "Cost API test passed"

REM Test recommendations API
call :print_status "Testing recommendations API..."
curl -f http://localhost:8080/api/recommendations/default >nul 2>&1
if errorlevel 1 (
    call :print_error "Recommendations API test failed"
    exit /b 1
)
call :print_success "Recommendations API test passed"

REM Test frontend
call :print_status "Testing frontend..."
curl -f http://localhost:3000 >nul 2>&1
if errorlevel 1 (
    call :print_error "Frontend test failed"
    exit /b 1
)
call :print_success "Frontend test passed"

call :print_success "All tests passed!"
goto :eof

REM Function to show logs
:show_logs
call :print_header "Application Logs"

cd /d "%SCRIPT_DIR%"

echo %CYAN%Backend Logs:%NC%
docker-compose -f %DOCKER_COMPOSE_FILE% logs --tail=20 backend

echo.
echo %CYAN%Frontend Logs:%NC%
docker-compose -f %DOCKER_COMPOSE_FILE% logs --tail=20 frontend

echo.
echo %CYAN%Database Logs:%NC%
docker-compose -f %DOCKER_COMPOSE_FILE% logs --tail=10 postgres
goto :eof

REM Function to stop the application
:stop_application
call :print_header "Stopping Application"

cd /d "%SCRIPT_DIR%"

call :print_status "Stopping all services..."
docker-compose -f %DOCKER_COMPOSE_FILE% down

call :print_success "Application stopped"
goto :eof

REM Function to clean up
:cleanup
call :print_header "Cleaning Up"

cd /d "%SCRIPT_DIR%"

call :print_status "Stopping and removing containers..."
docker-compose -f %DOCKER_COMPOSE_FILE% down -v

call :print_status "Removing volumes..."
docker volume rm k8s-kost_postgres_data k8s-kost_redis_data 2>nul

call :print_status "Removing images..."
docker rmi k8s-cost-optimizer:latest 2>nul

call :print_success "Cleanup complete"
goto :eof

REM Function to restart the application
:restart_application
call :print_header "Restarting Application"

call :stop_application
timeout /t 2 /nobreak >nul
call :start_application
goto :eof

REM Function to show help
:show_help
echo %PURPLE%Kubernetes Cost Optimizer - Quick Test Script (Windows)%NC%
echo.
echo %CYAN%Usage:%NC% %~nx0 [COMMAND]
echo.
echo %CYAN%Commands:%NC%
echo   %GREEN%start%NC%     - Start the application with sample data
echo   %GREEN%stop%NC%      - Stop the application
echo   %GREEN%restart%NC%   - Restart the application
echo   %GREEN%status%NC%    - Check application status
echo   %GREEN%test%NC%      - Run quick tests
echo   %GREEN%logs%NC%      - Show application logs
echo   %GREEN%cleanup%NC%   - Stop and clean up everything
echo   %GREEN%check%NC%     - Check prerequisites
echo   %GREEN%help%NC%      - Show this help message
echo.
echo %CYAN%Examples:%NC%
echo   %~nx0 start      # Start the application
echo   %~nx0 status     # Check if everything is running
echo   %~nx0 test       # Run tests to verify functionality
echo   %~nx0 logs       # View application logs
echo   %~nx0 cleanup    # Clean up everything
goto :eof

REM Main script logic
if "%1"=="" goto :show_help
if "%1"=="help" goto :show_help
if "%1"=="start" goto :start_command
if "%1"=="stop" goto :stop_application
if "%1"=="restart" goto :restart_application
if "%1"=="status" goto :status_command
if "%1"=="test" goto :run_tests
if "%1"=="logs" goto :show_logs
if "%1"=="cleanup" goto :cleanup
if "%1"=="check" goto :check_prerequisites
goto :show_help

:start_command
call :check_prerequisites
call :start_application
timeout /t 5 /nobreak >nul
call :check_status
call :show_access_info
goto :eof

:status_command
call :check_status
call :show_access_info
goto :eof 