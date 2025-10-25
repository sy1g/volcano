#!/bin/bash

# Volcano VAP/MAP Policy Validation Script
# This script provides test resources and validation scenarios for the migrated policies

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$SCRIPT_DIR/tests"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Create test directory if it doesn't exist
mkdir -p "$TEST_DIR"

# Generate test resources for each webhook type
generate_test_resources() {
    log_info "Generating test resources..."
    
    # HyperNode test resources
    cat > "$TEST_DIR/hypernode-valid.yaml" << 'EOF'
apiVersion: topology.volcano.sh/v1alpha1
kind: HyperNode
metadata:
  name: test-hypernode-valid
  namespace: default
spec:
  members:
  - selector:
      exactMatch:
        name: "worker-node-1"
  - selector:
      regexMatch:
        pattern: "gpu-node-.*"
EOF

    cat > "$TEST_DIR/hypernode-invalid.yaml" << 'EOF'
apiVersion: topology.volcano.sh/v1alpha1
kind: HyperNode
metadata:
  name: test-hypernode-invalid
  namespace: default
spec:
  members:
  - selector:
      exactMatch:
        name: "worker-node-1"
      regexMatch:
        pattern: "gpu-node-.*"  # Invalid: multiple selectors
EOF

    # JobFlow test resources
    cat > "$TEST_DIR/jobflow-valid.yaml" << 'EOF'
apiVersion: flow.volcano.sh/v1alpha1
kind: JobFlow
metadata:
  name: test-jobflow-valid
  namespace: default
spec:
  flows:
  - name: job-a
  - name: job-b
    dependsOn:
      targets: ["job-a"]
  - name: job-c
    dependsOn:
      targets: ["job-a", "job-b"]
EOF

    cat > "$TEST_DIR/jobflow-invalid.yaml" << 'EOF'
apiVersion: flow.volcano.sh/v1alpha1
kind: JobFlow
metadata:
  name: test-jobflow-invalid
  namespace: default
spec:
  flows:
  - name: job-a
    dependsOn:
      targets: ["job-b"]  # Invalid: circular dependency
  - name: job-b
    dependsOn:
      targets: ["job-a"]
EOF

    # Job test resources
    cat > "$TEST_DIR/job-valid.yaml" << 'EOF'
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job-valid
  namespace: default
spec:
  minAvailable: 2
  tasks:
  - name: worker
    replicas: 3
    minAvailable: 2
    template:
      spec:
        containers:
        - name: worker
          image: busybox
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
EOF

    cat > "$TEST_DIR/job-invalid.yaml" << 'EOF'
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job-invalid
  namespace: default
spec:
  minAvailable: -1  # Invalid: negative value
  tasks:
  - name: worker
    replicas: 3
    minAvailable: 5  # Invalid: greater than replicas
    template:
      spec:
        containers:
        - name: worker
          image: busybox
EOF

    # PodGroup test resources
    cat > "$TEST_DIR/podgroup-valid.yaml" << 'EOF'
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: test-podgroup-valid
  namespace: default
spec:
  minMember: 3
  maxMember: 5
  queue: "default"
EOF

    cat > "$TEST_DIR/podgroup-invalid.yaml" << 'EOF'
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: test-podgroup-invalid
  namespace: default
spec:
  minMember: 5
  maxMember: 3  # Invalid: max < min
  queue: "invalid-queue-name_with_underscores"  # Invalid: bad format
EOF

    # Pod test resources
    cat > "$TEST_DIR/pod-valid.yaml" << 'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-valid
  namespace: default
  annotations:
    scheduling.volcano.sh/job-name: "test-job"
    scheduling.volcano.sh/task-name: "worker"
    scheduling.volcano.sh/queue-name: "default"
spec:
  schedulerName: volcano
  containers:
  - name: worker
    image: busybox
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
EOF

    cat > "$TEST_DIR/pod-invalid.yaml" << 'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-invalid
  namespace: default
  annotations:
    scheduling.volcano.sh/job-name: "invalid_job_name"  # Invalid: underscores
    scheduling.volcano.sh/task-name: ""  # Invalid: empty
spec:
  schedulerName: default-scheduler  # Invalid: not volcano scheduler
  containers:
  - name: worker
    image: busybox
    # Missing resource requests for volcano pod
EOF

    # Queue test resources
    cat > "$TEST_DIR/queue-valid.yaml" << 'EOF'
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: test-queue-valid
  annotations:
    scheduling.volcano.sh/hierarchy: "engineering/ml"
    scheduling.volcano.sh/hierarchy-weights: "2/3"
spec:
  weight: 5
  state: Open
  capability:
    cpu: "100"
    memory: "200Gi"
EOF

    cat > "$TEST_DIR/queue-invalid.yaml" << 'EOF'
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: test-queue-invalid
  annotations:
    scheduling.volcano.sh/hierarchy: "engineering/ml/team"
    scheduling.volcano.sh/hierarchy-weights: "2/3"  # Invalid: length mismatch
spec:
  weight: 0  # Invalid: weight must be > 0
  state: InvalidState  # Invalid: not a valid state
EOF

    log_success "Test resources generated in $TEST_DIR"
}

# Test a specific resource type
test_resource_type() {
    local resource_type="$1"
    local dry_run="${2:-true}"
    
    log_info "Testing $resource_type resources..."
    
    local valid_file="$TEST_DIR/${resource_type}-valid.yaml"
    local invalid_file="$TEST_DIR/${resource_type}-invalid.yaml"
    
    if [[ -f "$valid_file" ]]; then
        log_info "Testing valid $resource_type..."
        if [[ "$dry_run" == "true" ]]; then
            if kubectl apply --dry-run=server -f "$valid_file" &>/dev/null; then
                log_success "Valid $resource_type passed validation"
            else
                log_error "Valid $resource_type failed validation (unexpected)"
                kubectl apply --dry-run=server -f "$valid_file"
            fi
        else
            kubectl apply -f "$valid_file"
            log_success "Valid $resource_type created"
        fi
    fi
    
    if [[ -f "$invalid_file" ]]; then
        log_info "Testing invalid $resource_type..."
        if [[ "$dry_run" == "true" ]]; then
            if kubectl apply --dry-run=server -f "$invalid_file" &>/dev/null; then
                log_warning "Invalid $resource_type passed validation (may indicate policy issue)"
            else
                log_success "Invalid $resource_type correctly rejected"
            fi
        else
            if kubectl apply -f "$invalid_file" &>/dev/null; then
                log_warning "Invalid $resource_type was created (policy may not be enforcing)"
            else
                log_success "Invalid $resource_type correctly rejected"
            fi
        fi
    fi
}

# Run comprehensive tests
run_tests() {
    local dry_run="${1:-true}"
    
    log_info "=== RUNNING COMPREHENSIVE POLICY TESTS ==="
    
    if [[ "$dry_run" == "false" ]]; then
        log_warning "Running tests in APPLY mode - resources will be created"
        read -p "Continue? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Test cancelled"
            return 0
        fi
    else
        log_info "Running tests in DRY-RUN mode"
    fi
    
    # Test each resource type
    test_resource_type "hypernode" "$dry_run"
    echo
    test_resource_type "jobflow" "$dry_run"
    echo
    test_resource_type "job" "$dry_run"
    echo
    test_resource_type "podgroup" "$dry_run"
    echo
    test_resource_type "pod" "$dry_run"
    echo
    test_resource_type "queue" "$dry_run"
    
    log_success "All tests completed"
}

# Clean up test resources
cleanup_tests() {
    log_info "Cleaning up test resources..."
    
    # Delete test resources if they exist
    kubectl delete hypernode,jobflow,job,podgroup,pod,queue -l "test=volcano-vap-map" --ignore-not-found=true
    
    # Delete specific test resources
    for resource_file in "$TEST_DIR"/*-valid.yaml; do
        if [[ -f "$resource_file" ]]; then
            kubectl delete -f "$resource_file" --ignore-not-found=true
        fi
    done
    
    log_success "Test cleanup completed"
}

# Show policy effectiveness report
policy_report() {
    log_info "=== POLICY EFFECTIVENESS REPORT ==="
    
    echo
    log_info "Deployed Policies:"
    kubectl get validatingadmissionpolicy,mutatingadmissionpolicy -l "volcano.sh/migration" -o custom-columns="NAME:.metadata.name,TYPE:.kind,AGE:.metadata.creationTimestamp" 2>/dev/null || log_warning "No policies found"
    
    echo
    log_info "Policy Violations (last 50 events):"
    kubectl get events --field-selector reason=PolicyViolation --sort-by='.lastTimestamp' | tail -50 || log_warning "No policy violation events found"
    
    echo
    log_info "Admission Performance (webhook vs policy comparison):"
    log_info "Check volcano-admission webhook logs vs API server logs for latency comparison"
    
    echo
    log_info "Resource Admission Statistics:"
    log_info "Monitor successful vs rejected admission requests in your monitoring system"
}

# Benchmark policy performance
benchmark_performance() {
    local iterations="${1:-10}"
    
    log_info "=== PERFORMANCE BENCHMARK ==="
    log_info "Testing admission latency with $iterations iterations"
    
    local total_time=0
    local successful=0
    
    for ((i=1; i<=iterations; i++)); do
        local start_time=$(date +%s%N)
        
        if kubectl apply --dry-run=server -f "$TEST_DIR/job-valid.yaml" &>/dev/null; then
            local end_time=$(date +%s%N)
            local duration=$(( (end_time - start_time) / 1000000 ))  # Convert to milliseconds
            total_time=$((total_time + duration))
            successful=$((successful + 1))
            log_info "Iteration $i: ${duration}ms"
        else
            log_warning "Iteration $i: Failed"
        fi
    done
    
    if [[ $successful -gt 0 ]]; then
        local average=$((total_time / successful))
        log_success "Average admission latency: ${average}ms (${successful}/${iterations} successful)"
    else
        log_error "No successful admission requests"
    fi
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS] COMMAND"
    echo
    echo "Commands:"
    echo "  generate         Generate test resources"
    echo "  test            Run comprehensive validation tests (dry-run by default)"
    echo "  test-apply      Run tests and actually create resources"
    echo "  cleanup         Clean up test resources"
    echo "  report          Show policy effectiveness report"
    echo "  benchmark       Run performance benchmark"
    echo
    echo "Options:"
    echo "  --help          Show this help message"
    echo
    echo "Examples:"
    echo "  $0 generate                  # Generate test resources"
    echo "  $0 test                      # Run dry-run tests"
    echo "  $0 test-apply                # Run tests with actual resource creation"
    echo "  $0 benchmark 20              # Run 20 iterations of performance test"
    echo "  $0 report                    # Show policy effectiveness report"
}

# Main script logic
main() {
    local command=""
    local iterations=10
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help)
                show_usage
                exit 0
                ;;
            generate|test|test-apply|cleanup|report|benchmark)
                command="$1"
                shift
                if [[ "$command" == "benchmark" && $# -gt 0 && $1 =~ ^[0-9]+$ ]]; then
                    iterations="$1"
                    shift
                fi
                break
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    if [[ -z "$command" ]]; then
        log_error "No command specified"
        show_usage
        exit 1
    fi
    
    # Execute command
    case "$command" in
        generate)
            generate_test_resources
            ;;
        test)
            generate_test_resources
            run_tests true
            ;;
        test-apply)
            generate_test_resources
            run_tests false
            ;;
        cleanup)
            cleanup_tests
            ;;
        report)
            policy_report
            ;;
        benchmark)
            generate_test_resources
            benchmark_performance "$iterations"
            ;;
        *)
            log_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"