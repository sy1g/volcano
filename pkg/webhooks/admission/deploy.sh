#!/bin/bash

# Volcano Webhook to VAP/MAP Migration Deployment Script
# This script helps deploy ValidatingAdmissionPolicy and MutatingAdmissionPolicy
# configurations for Volcano webhooks in a phased approach

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
POLICIES_DIR="$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    # Check Kubernetes version
    local k8s_version
    k8s_version=$(kubectl version --client -o json | jq -r '.clientVersion.major + "." + .clientVersion.minor' 2>/dev/null || echo "unknown")
    log_info "Kubernetes client version: $k8s_version"
    
    # Check for ValidatingAdmissionPolicy support
    if ! kubectl api-resources | grep -q "validatingadmissionpolicies"; then
        log_error "ValidatingAdmissionPolicy not supported. Requires Kubernetes 1.30+"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Deploy a specific policy file
deploy_policy() {
    local policy_file="$1"
    local dry_run="${2:-false}"
    
    if [[ "$dry_run" == "true" ]]; then
        log_info "Dry-run: Would deploy $policy_file"
        kubectl apply --dry-run=client -f "$policy_file"
    else
        log_info "Deploying $policy_file"
        kubectl apply -f "$policy_file"
        log_success "Deployed $policy_file"
    fi
}

# Phase 1: High-feasibility migrations
deploy_phase1() {
    local dry_run="${1:-false}"
    
    log_info "=== PHASE 1: High-Feasibility Migrations ==="
    log_info "Deploying policies with 95%+ migration confidence"
    
    # HyperNodes validation (95% confidence)
    if [[ -f "$POLICIES_DIR/hypernodes/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/hypernodes/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    # Jobs mutation (90% confidence)
    if [[ -f "$POLICIES_DIR/jobs/policies/mutating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/jobs/policies/mutating-admission-policy.yaml" "$dry_run"
    fi
    
    # Queues mutation (85% confidence)
    if [[ -f "$POLICIES_DIR/queues/policies/mutating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/queues/policies/mutating-admission-policy.yaml" "$dry_run"
    fi
    
    # Pods validation (85% confidence)
    if [[ -f "$POLICIES_DIR/pods/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/pods/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    log_success "Phase 1 deployment completed"
}

# Phase 2: Medium-feasibility migrations
deploy_phase2() {
    local dry_run="${1:-false}"
    
    log_info "=== PHASE 2: Medium-Feasibility Migrations ==="
    log_info "Deploying policies with 60-80% migration confidence"
    log_warning "These policies start in 'Warn' mode for gradual rollout"
    
    # JobFlows validation (70% confidence)
    if [[ -f "$POLICIES_DIR/jobflows/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/jobflows/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    # Jobs validation (65% confidence)
    if [[ -f "$POLICIES_DIR/jobs/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/jobs/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    # Pods mutation (70% confidence)
    if [[ -f "$POLICIES_DIR/pods/policies/mutating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/pods/policies/mutating-admission-policy.yaml" "$dry_run"
    fi
    
    log_success "Phase 2 deployment completed"
}

# Phase 3: Low-feasibility (demonstration only)
deploy_phase3() {
    local dry_run="${1:-false}"
    
    log_info "=== PHASE 3: Low-Feasibility Migrations ==="
    log_warning "These policies have limited functionality and are for demonstration only"
    log_warning "Consider controller-based validation for full functionality"
    
    # PodGroups policies (30-60% confidence)
    if [[ -f "$POLICIES_DIR/podgroups/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/podgroups/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    if [[ -f "$POLICIES_DIR/podgroups/policies/mutating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/podgroups/policies/mutating-admission-policy.yaml" "$dry_run"
    fi
    
    # Queues validation (25% confidence)
    if [[ -f "$POLICIES_DIR/queues/policies/validating-admission-policy.yaml" ]]; then
        deploy_policy "$POLICIES_DIR/queues/policies/validating-admission-policy.yaml" "$dry_run"
    fi
    
    log_success "Phase 3 deployment completed"
}

# Remove all policies
remove_policies() {
    log_info "=== REMOVING ALL POLICIES ==="
    log_warning "This will remove all ValidatingAdmissionPolicy and MutatingAdmissionPolicy resources"
    
    read -p "Are you sure you want to remove all policies? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Operation cancelled"
        return 0
    fi
    
    # Remove all policies with volcano labels
    kubectl delete validatingadmissionpolicy -l "volcano.sh/migration=vap" --ignore-not-found=true
    kubectl delete mutatingadmissionpolicy -l "volcano.sh/migration=map" --ignore-not-found=true
    kubectl delete validatingadmissionpolicybinding -l "volcano.sh/migration=vap" --ignore-not-found=true
    kubectl delete mutatingadmissionpolicybinding -l "volcano.sh/migration=map" --ignore-not-found=true
    
    log_success "All policies removed"
}

# Monitor policy status
monitor_policies() {
    log_info "=== POLICY STATUS MONITORING ==="
    
    echo
    log_info "ValidatingAdmissionPolicies:"
    kubectl get validatingadmissionpolicy -l "volcano.sh/migration=vap" -o wide 2>/dev/null || log_warning "No VAPs found"
    
    echo
    log_info "MutatingAdmissionPolicies:"
    kubectl get mutatingadmissionpolicy -l "volcano.sh/migration=map" -o wide 2>/dev/null || log_warning "No MAPs found"
    
    echo
    log_info "Policy Bindings:"
    kubectl get validatingadmissionpolicybinding,mutatingadmissionpolicybinding -l "volcano.sh/migration" -o wide 2>/dev/null || log_warning "No policy bindings found"
    
    echo
    log_info "Recent Policy Violation Events:"
    kubectl get events --field-selector reason=PolicyViolation --sort-by='.lastTimestamp' | tail -10 || log_warning "No policy violation events found"
}

# Update validation action for gradual rollout
update_validation_action() {
    local action="$1"
    
    case "$action" in
        "warn"|"audit"|"deny")
            log_info "Updating all VAP bindings to use '$action' validation action"
            
            # Get all VAP bindings and update them
            local bindings
            bindings=$(kubectl get validatingadmissionpolicybinding -l "volcano.sh/migration=vap" -o name 2>/dev/null || true)
            
            if [[ -z "$bindings" ]]; then
                log_warning "No ValidatingAdmissionPolicyBinding resources found"
                return 0
            fi
            
            for binding in $bindings; do
                log_info "Updating $binding"
                kubectl patch "$binding" --type='merge' -p='{"spec":{"validationActions":["'$(echo "$action" | tr '[:lower:]' '[:upper:]')'"]}}' 
            done
            
            log_success "Updated validation action to '$action'"
            ;;
        *)
            log_error "Invalid validation action. Use: warn, audit, or deny"
            exit 1
            ;;
    esac
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS] COMMAND"
    echo
    echo "Commands:"
    echo "  phase1           Deploy high-feasibility migrations (95%+ confidence)"
    echo "  phase2           Deploy medium-feasibility migrations (60-80% confidence)"
    echo "  phase3           Deploy low-feasibility migrations (demonstration only)"
    echo "  all              Deploy all phases"
    echo "  remove           Remove all deployed policies"
    echo "  monitor          Show status of deployed policies"
    echo "  set-action       Update validation action for gradual rollout"
    echo
    echo "Options:"
    echo "  --dry-run        Show what would be deployed without applying"
    echo "  --help           Show this help message"
    echo
    echo "Examples:"
    echo "  $0 phase1                    # Deploy high-confidence policies"
    echo "  $0 --dry-run phase2          # Preview medium-confidence policies"
    echo "  $0 set-action warn           # Set all policies to warn mode"
    echo "  $0 set-action deny           # Set all policies to enforcement mode"
    echo "  $0 monitor                   # Check policy status"
}

# Main script logic
main() {
    local dry_run=false
    local command=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                dry_run=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            phase1|phase2|phase3|all|remove|monitor|set-action)
                command="$1"
                shift
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
    
    # Check prerequisites for deployment commands
    if [[ "$command" =~ ^(phase1|phase2|phase3|all|remove|monitor|set-action)$ ]]; then
        check_prerequisites
    fi
    
    # Execute command
    case "$command" in
        phase1)
            deploy_phase1 "$dry_run"
            ;;
        phase2)
            deploy_phase2 "$dry_run"
            ;;
        phase3)
            deploy_phase3 "$dry_run"
            ;;
        all)
            deploy_phase1 "$dry_run"
            echo
            deploy_phase2 "$dry_run"
            echo
            deploy_phase3 "$dry_run"
            ;;
        remove)
            remove_policies
            ;;
        monitor)
            monitor_policies
            ;;
        set-action)
            if [[ $# -eq 0 ]]; then
                log_error "Validation action required. Use: warn, audit, or deny"
                exit 1
            fi
            update_validation_action "$1"
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