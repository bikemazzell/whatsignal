#!/bin/bash

# WhatsSignal Integration Test Runner
# Supports running tests with or without Docker dependencies

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCKER_COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default settings
USE_DOCKER=false
CLEANUP_AFTER=true
VERBOSE=false
TEST_PATTERN=""
TIMEOUT="10m"
PARALLEL=false

# Print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS] [TEST_PATTERN]

WhatsSignal Integration Test Runner

OPTIONS:
    -d, --docker        Use Docker services for external dependencies
    -n, --no-cleanup    Don't cleanup Docker services after tests
    -v, --verbose       Enable verbose output
    -t, --timeout DURATION  Test timeout (default: 10m)
    -p, --parallel      Run tests in parallel
    -h, --help          Show this help message

TEST_PATTERN:
    Optional Go test pattern to run specific tests (e.g., "TestEndToEnd")

EXAMPLES:
    $0                                  # Run all integration tests with mocks
    $0 -d                              # Run with Docker services
    $0 -d -v TestMultiChannel          # Run specific test with Docker and verbose output
    $0 --docker --no-cleanup           # Keep Docker services running after tests

DOCKER SERVICES:
    When --docker is used, the following services are available:
    - Signal-CLI (port 8080)
    - WAHA WhatsApp API (port 3000)
    - PostgreSQL (port 5432)
    - Redis (port 6379)
    - Prometheus (port 9090)
    - Jaeger (port 16686)
    - MinIO (port 9000)
    - Nginx (port 8081)

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--docker)
            USE_DOCKER=true
            shift
            ;;
        -n|--no-cleanup)
            CLEANUP_AFTER=false
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -p|--parallel)
            PARALLEL=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "Unknown option $1"
            usage
            exit 1
            ;;
        *)
            TEST_PATTERN="$1"
            shift
            ;;
    esac
done

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    if [[ "$USE_DOCKER" == true ]]; then
        if ! command -v docker &> /dev/null; then
            log_error "Docker is not installed or not in PATH"
            exit 1
        fi
        
        if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
            log_error "Docker Compose is not available"
            exit 1
        fi
    fi
    
    log_success "Dependencies check passed"
}

# Setup Docker services
setup_docker() {
    if [[ "$USE_DOCKER" != true ]]; then
        return 0
    fi
    
    log_info "Setting up Docker services..."
    
    cd "$SCRIPT_DIR"
    
    # Check if docker-compose or docker compose should be used
    if command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE_CMD="docker-compose"
    else
        DOCKER_COMPOSE_CMD="docker compose"
    fi
    
    # Start services
    $DOCKER_COMPOSE_CMD -f "$DOCKER_COMPOSE_FILE" up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    
    local max_wait=120
    local wait_time=0
    
    while [[ $wait_time -lt $max_wait ]]; do
        if $DOCKER_COMPOSE_CMD -f "$DOCKER_COMPOSE_FILE" ps | grep -q "Up (healthy)"; then
            log_success "Docker services are ready"
            return 0
        fi
        
        log_info "Waiting for services... ($wait_time/$max_wait seconds)"
        sleep 5
        wait_time=$((wait_time + 5))
    done
    
    log_warning "Some services may not be fully ready, continuing anyway..."
    
    # Show service status
    if [[ "$VERBOSE" == true ]]; then
        log_info "Service status:"
        $DOCKER_COMPOSE_CMD -f "$DOCKER_COMPOSE_FILE" ps
    fi
}

# Cleanup Docker services
cleanup_docker() {
    if [[ "$USE_DOCKER" != true ]] || [[ "$CLEANUP_AFTER" != true ]]; then
        return 0
    fi

    log_info "Cleaning up Docker services..."

    cd "$SCRIPT_DIR"

    # Use the same detection logic as in setup_docker
    if command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE_CMD="docker-compose"
    else
        DOCKER_COMPOSE_CMD="docker compose"
    fi

    $DOCKER_COMPOSE_CMD -f "$DOCKER_COMPOSE_FILE" down -v

    log_success "Docker cleanup completed"
}

# Set environment variables for tests
setup_test_environment() {
    log_info "Setting up test environment..."
    
    export WHATSIGNAL_TEST_MODE=true
    export WHATSIGNAL_LOG_LEVEL=info
    
    if [[ "$USE_DOCKER" == true ]]; then
        # Set environment variables to use Docker services
        export WHATSIGNAL_WHATSAPP_API_BASE_URL="http://localhost:3000"
        export WHATSIGNAL_SIGNAL_RPC_URL="http://localhost:8080"
        export WHATSIGNAL_DATABASE_PATH="/tmp/whatsignal-integration-test.db"
        export WHATSIGNAL_INTEGRATION_USE_DOCKER=true
    else
        # Use mock services
        export WHATSIGNAL_INTEGRATION_USE_DOCKER=false
    fi
    
    if [[ "$VERBOSE" == true ]]; then
        export WHATSIGNAL_LOG_LEVEL=debug
        export WHATSIGNAL_INTEGRATION_VERBOSE=true
    fi
    
    log_success "Test environment configured"
}

# Run integration tests
run_tests() {
    log_info "Running integration tests..."
    
    cd "$PROJECT_ROOT"
    
    # Build test command
    local test_cmd="go test"
    
    if [[ "$VERBOSE" == true ]]; then
        test_cmd="$test_cmd -v"
    fi
    
    if [[ "$PARALLEL" == true ]]; then
        test_cmd="$test_cmd -parallel 4"
    fi
    
    test_cmd="$test_cmd -timeout $TIMEOUT"
    test_cmd="$test_cmd -race"
    test_cmd="$test_cmd ./integration_test/..."
    
    if [[ -n "$TEST_PATTERN" ]]; then
        test_cmd="$test_cmd -run $TEST_PATTERN"
    fi
    
    log_info "Executing: $test_cmd"
    
    if $test_cmd; then
        log_success "All integration tests passed!"
        return 0
    else
        log_error "Some integration tests failed!"
        return 1
    fi
}

# Main execution
main() {
    log_info "WhatsSignal Integration Test Runner"
    log_info "Docker mode: $USE_DOCKER"
    log_info "Test pattern: ${TEST_PATTERN:-'all tests'}"
    log_info "Cleanup after: $CLEANUP_AFTER"
    
    # Trap to ensure cleanup on exit
    if [[ "$USE_DOCKER" == true ]]; then
        trap cleanup_docker EXIT
    fi
    
    check_dependencies
    setup_test_environment
    setup_docker
    
    if run_tests; then
        log_success "Integration tests completed successfully!"
        exit 0
    else
        log_error "Integration tests failed!"
        exit 1
    fi
}

# Run main function
main "$@"