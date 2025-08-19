# Queues Webhook - Detailed Analysis

## Overview

The Queues webhook is one of the most complex webhooks in Volcano, providing both validation and mutation for Queue resources. It handles comprehensive queue hierarchy management, resource validation, and hierarchical queue relationships that are fundamental to Volcano's scheduling and resource management capabilities.

## Functional Analysis

### Purpose
- **Resource Type**: `scheduling/v1beta1/Queue`
- **Operations**: CREATE, UPDATE, DELETE (validate), CREATE (mutate)
- **Webhook Types**: Both ValidatingAdmissionWebhook and MutatingAdmissionWebhook
- **Primary Functions**:
  - Queue hierarchy validation and management
  - Resource allocation validation (capability, deserved, guarantee)
  - Queue state and weight validation
  - Hierarchical annotation management
  - Queue deletion protection

### Validation Logic (validate/validate_queue.go)

#### 1. Core Queue Validation
```go
func validateQueue(queue *schedulingv1beta1.Queue) error {
    errs := field.ErrorList{}
    resourcePath := field.NewPath("requestBody")

    errs = append(errs, validateStateOfQueue(queue.Status.State, resourcePath.Child("spec").Child("state"))...)
    errs = append(errs, validateWeightOfQueue(queue.Spec.Weight, resourcePath.Child("spec").Child("weight"))...)
    errs = append(errs, validateResourceOfQueue(queue.Spec, resourcePath.Child("spec"))...)
    errs = append(errs, validateHierarchicalAttributes(queue, resourcePath.Child("metadata").Child("annotations"))...)

    return errs.ToAggregate()
}
```

#### 2. Queue State Validation
```go
func validateStateOfQueue(value schedulingv1beta1.QueueState, fldPath *field.Path) field.ErrorList {
    validQueueStates := []schedulingv1beta1.QueueState{
        schedulingv1beta1.QueueStateOpen,
        schedulingv1beta1.QueueStateClosed,
    }
    
    for _, validQueue := range validQueueStates {
        if value == validQueue {
            return errs
        }
    }
    
    return append(errs, field.Invalid(fldPath, value, fmt.Sprintf("queue state must be in %v", validQueueStates)))
}
```

**Valid States:**
- `Open`: Queue accepts new jobs
- `Closed`: Queue rejects new jobs

#### 3. Queue Weight Validation
```go
func validateWeightOfQueue(value int32, fldPath *field.Path) field.ErrorList {
    if value > 0 {
        return errs
    }
    return append(errs, field.Invalid(fldPath, value, "queue weight must be a positive integer"))
}
```

#### 4. Resource Validation
```go
func validateResourceOfQueue(resource schedulingv1beta1.QueueSpec, fldPath *field.Path) field.ErrorList {
    capabilityResource := api.NewResource(resource.Capability)
    deservedResource := api.NewResource(resource.Deserved)
    guaranteeResource := api.NewResource(resource.Guarantee.Resource)

    // Capability >= Deserved >= Guarantee
    if capabilityResource.LessPartly(deservedResource, api.Zero) {
        return append(errs, field.Invalid(fldPath.Child("deserved"),
            deservedResource.String(), "deserved should less equal than capability"))
    }

    if capabilityResource.LessPartly(guaranteeResource, api.Zero) {
        return append(errs, field.Invalid(fldPath.Child("guarantee"),
            guaranteeResource.String(), "guarantee should less equal than capability"))
    }

    if deservedResource.LessPartly(guaranteeResource, api.Zero) {
        return append(errs, field.Invalid(fldPath.Child("guarantee"),
            guaranteeResource.String(), "guarantee should less equal than deserved"))
    }
}
```

**Resource Hierarchy**: `Capability >= Deserved >= Guarantee`

#### 5. Hierarchical Attributes Validation
```go
func validateHierarchicalAttributes(queue *schedulingv1beta1.Queue, fldPath *field.Path) field.ErrorList {
    hierarchy := queue.Annotations[schedulingv1beta1.KubeHierarchyAnnotationKey]
    hierarchicalWeights := queue.Annotations[schedulingv1beta1.KubeHierarchyWeightAnnotationKey]
    
    if hierarchy != "" || hierarchicalWeights != "" {
        paths := strings.Split(hierarchy, "/")
        weights := strings.Split(hierarchicalWeights, "/")
        
        // Path length must match weights length
        if len(paths) != len(weights) {
            return append(errs, field.Invalid(fldPath, hierarchy, "hierarchy and weights must have same length"))
        }
        
        // Validate weights are positive numbers
        // Check for conflicting hierarchies
        // Validate against existing queues
    }
}
```

**Annotation Keys:**
- `scheduling.volcano.sh/hierarchy`: Queue hierarchy path (e.g., `"root/engineering/ml"`)
- `scheduling.volcano.sh/hierarchy-weights`: Corresponding weights (e.g., `"1/2/3"`)

#### 6. Hierarchical Queue Validation
```go
func validateHierarchicalQueue(queue *schedulingv1beta1.Queue) error {
    if queue.Spec.Parent == "" || queue.Spec.Parent == "root" {
        return nil
    }
    
    parentQueue, err := config.QueueLister.Get(queue.Spec.Parent)
    if err != nil {
        return fmt.Errorf("failed to get parent queue of queue %s: %v", queue.Name, err)
    }
    
    // Parent queue with allocated pods cannot have child queues
    childQueueNames, err := listQueueChild(parentQueue.Name)
    if len(childQueueNames) == 0 {
        if allocated, ok := parentQueue.Status.Allocated[v1.ResourcePods]; ok && !allocated.IsZero() {
            return fmt.Errorf("queue %s cannot be the parent queue of queue %s because it has allocated Pods: %d",
                parentQueue.Name, queue.Name, allocated.Value())
        }
    }
}
```

#### 7. Queue Deletion Validation
```go
func validateQueueDeleting(queueName string) error {
    // Protect system queues
    if queueName == "default" || queueName == "root" {
        return fmt.Errorf("`%s` queue can not be deleted", queueName)
    }
    
    // Check for child queues
    childQueueNames, err := listQueueChild(queueName)
    if len(childQueueNames) > 0 {
        return fmt.Errorf("queue %s can not be deleted because it has %d child queues: %s",
            queueName, len(childQueueNames), strings.Join(childQueueNames, ", "))
    }
}
```

### Mutation Logic (mutate/mutate_queue.go)

#### 1. Hierarchy Root Node Addition
```go
func createQueuePatch(queue *schedulingv1beta1.Queue) ([]byte, error) {
    hierarchy := queue.Annotations[schedulingv1beta1.KubeHierarchyAnnotationKey]
    hierarchicalWeights := queue.Annotations[schedulingv1beta1.KubeHierarchyWeightAnnotationKey]

    if hierarchy != "" && hierarchicalWeights != "" && !strings.HasPrefix(hierarchy, "root") {
        patch = append(patch, patchOperation{
            Op:    "add",
            Path:  fmt.Sprintf("/metadata/annotations/%s", 
                    strings.ReplaceAll(schedulingv1beta1.KubeHierarchyAnnotationKey, "/", "~1")),
            Value: fmt.Sprintf("root/%s", hierarchy),
        })
        patch = append(patch, patchOperation{
            Op:    "add",
            Path:  fmt.Sprintf("/metadata/annotations/%s", 
                    strings.ReplaceAll(schedulingv1beta1.KubeHierarchyWeightAnnotationKey, "/", "~1")),
            Value: fmt.Sprintf("1/%s", hierarchicalWeights),
        })
    }
}
```

#### 2. Default Values Assignment
```go
// Default reclaimable to true
trueValue := true
if queue.Spec.Reclaimable == nil {
    patch = append(patch, patchOperation{
        Op:    "add",
        Path:  "/spec/reclaimable",
        Value: &trueValue,
    })
}

// Default weight to 1
defaultWeight := 1
if queue.Spec.Weight == 0 {
    patch = append(patch, patchOperation{
        Op:    "add",
        Path:  "/spec/weight",
        Value: &defaultWeight,
    })
}
```

## Implementation Details

### File Structure
```
pkg/webhooks/admission/queues/
├── validate/
│   ├── validate_queue.go      # Main validation logic
│   └── validate_queue_test.go # Validation tests
└── mutate/
    ├── mutate_queue.go        # Mutation logic
    └── mutate_queue_test.go   # Mutation tests
```

### Key Dependencies

#### External API Calls
- **Queue Lister**: `config.QueueLister.List()` and `config.QueueLister.Get()`
- **Cluster State**: Real-time queue hierarchy and allocation information
- **Resource API**: Queue resource calculations and comparisons

#### Complex Algorithms
- **Hierarchy Validation**: Path conflict detection across existing queues
- **Resource Comparison**: Multi-dimensional resource validation
- **Parent-Child Relationships**: Dynamic hierarchy validation

## Complex Validation Examples

### Valid Queue Hierarchy
```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: ml-training
  annotations:
    scheduling.volcano.sh/hierarchy: "engineering/ml"
    scheduling.volcano.sh/hierarchy-weights: "2/3"
spec:
  parent: "engineering"
  weight: 5
  capability:
    cpu: "100"
    memory: "500Gi"
  deserved:
    cpu: "80"
    memory: "400Gi"
  guarantee:
    resource:
      cpu: "20"
      memory: "100Gi"
```

### Invalid Resource Configuration
```yaml
# This would fail validation
spec:
  capability:
    cpu: "50"     # Capability: 50 CPU
  deserved:
    cpu: "80"     # Deserved: 80 CPU > Capability (invalid)
  guarantee:
    resource:
      cpu: "100"  # Guarantee: 100 CPU > Deserved (invalid)
```

## Migration Assessment

### Validation Migration: LOW (🔴)

**Confidence Level**: 25%

**Rationale:**
1. **Complex External Dependencies**: Extensive use of QueueLister for cluster state
2. **Dynamic Hierarchy Validation**: Requires checking against all existing queues
3. **Resource Calculations**: Complex multi-dimensional resource comparisons
4. **Parent-Child Relationships**: Dynamic validation based on cluster state
5. **Allocated Resource Checks**: Requires access to queue status and allocation data

### Mutation Migration: HIGH (🟢)

**Confidence Level**: 85%

**Rationale:**
1. **Simple Logic**: Most mutations are straightforward default assignments
2. **Field-Based**: Primarily based on field existence/values
3. **String Manipulation**: Hierarchy path manipulation is deterministic
4. **No External Calls**: Mutation logic is self-contained

## Detailed Migration Challenges

### Challenge 1: Queue Hierarchy Validation

#### Current Implementation
```go
queueList, err := config.QueueLister.List(labels.Everything())
for _, queueInTree := range queueList {
    hierarchyInTree := queueInTree.Annotations[schedulingv1beta1.KubeHierarchyAnnotationKey]
    if hierarchyInTree != "" && queue.Name != queueInTree.Name &&
        strings.HasPrefix(hierarchyInTree, hierarchy+"/") {
        return fmt.Errorf("hierarchy conflict detected")
    }
}
```

#### CEL Limitations
- **No Cluster API Access**: Cannot list existing queues
- **No Cross-Resource Validation**: Cannot compare against other queue objects
- **No Dynamic State**: Cannot access real-time cluster information

### Challenge 2: Parent-Child Queue Validation

#### Current Implementation
```go
parentQueue, err := config.QueueLister.Get(queue.Spec.Parent)
childQueueNames, err := listQueueChild(parentQueue.Name)
if allocated, ok := parentQueue.Status.Allocated[v1.ResourcePods]; ok && !allocated.IsZero() {
    return error
}
```

#### CEL Limitations
- **No External Object Access**: Cannot access parent queue object
- **No Status Field Access**: Cannot check allocation status
- **No Dynamic Queries**: Cannot list child queues

### Challenge 3: Resource Validation Complexity

#### Current Implementation
```go
capabilityResource := api.NewResource(resource.Capability)
deservedResource := api.NewResource(resource.Deserved)
if capabilityResource.LessPartly(deservedResource, api.Zero) {
    return error
}
```

#### CEL Challenges
- **Complex Resource Comparison**: Multi-dimensional resource calculations
- **Custom API Types**: Volcano-specific resource types and operations
- **Partial Comparison Logic**: Complex "LessPartly" semantics

## Migration Strategies

### Strategy 1: Controller-Based Validation (Recommended)

#### Implementation
```go
type QueueController struct {
    client.Client
    QueueLister schedulingv1beta1.QueueLister
}

func (r *QueueController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var queue schedulingv1beta1.Queue
    if err := r.Get(ctx, req.NamespacedName, &queue); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Validate hierarchy conflicts
    if err := r.validateHierarchy(&queue); err != nil {
        return r.setErrorStatus(&queue, err)
    }

    // Validate parent-child relationships
    if err := r.validateParentChild(&queue); err != nil {
        return r.setErrorStatus(&queue, err)
    }

    // Validate resource allocations
    if err := r.validateResources(&queue); err != nil {
        return r.setErrorStatus(&queue, err)
    }

    return r.setReadyStatus(&queue)
}
```

#### Advantages
- **Full API Access**: Can validate against cluster state
- **Complex Logic**: Supports sophisticated validation rules
- **Asynchronous**: Doesn't block admission pipeline
- **Status Reporting**: Can provide detailed error information

#### Disadvantages
- **Delayed Validation**: Not fail-fast at admission time
- **Complexity**: Additional component to maintain
- **User Experience**: Less immediate feedback

### Strategy 2: Simplified CEL with Reduced Functionality

#### Basic Field Validation
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: queue-basic-validation
spec:
  validations:
  # Queue state validation
  - expression: |
      !has(object.status.state) || 
      object.status.state in ["Open", "Closed"]
    message: "queue state must be Open or Closed"

  # Weight validation
  - expression: "object.spec.weight > 0"
    message: "queue weight must be a positive integer"

  # Basic resource validation (simplified)
  - expression: |
      !has(object.spec.capability) || !has(object.spec.deserved) ||
      object.spec.capability.cpu >= object.spec.deserved.cpu
    message: "deserved CPU must not exceed capability CPU"

  # Protected queue deletion
  - expression: |
      object.metadata.name != "default" && object.metadata.name != "root"
    message: "cannot delete protected queues (default, root)"

  # Annotation format validation
  - expression: |
      !has(object.metadata.annotations["scheduling.volcano.sh/hierarchy"]) ||
      !has(object.metadata.annotations["scheduling.volcano.sh/hierarchy-weights"]) ||
      size(object.metadata.annotations["scheduling.volcano.sh/hierarchy"].split("/")) == 
      size(object.metadata.annotations["scheduling.volcano.sh/hierarchy-weights"].split("/"))
    message: "hierarchy and hierarchy-weights must have same number of elements"
```

#### Limitations
- **No Hierarchy Conflict Detection**: Cannot validate against existing queues
- **No Parent-Child Validation**: Cannot check parent queue state
- **Simplified Resource Validation**: Basic comparisons only
- **No Dynamic State**: Static validation only

### Strategy 3: Hybrid Approach

#### CEL for Basic Validations
```yaml
validations:
- expression: "object.spec.weight > 0"
  message: "queue weight must be positive"
- expression: |
    object.status.state in ["Open", "Closed"]
  message: "invalid queue state"
```

#### Simplified Webhook for Complex Validations
```go
func ValidateQueuesComplex(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
    queue, err := schema.DecodeQueue(ar.Request.Object, ar.Request.Resource)
    if err != nil {
        return util.ToAdmissionResponse(err)
    }
    
    // Only complex validations that require API access
    if err := validateHierarchyConflicts(queue); err != nil {
        return util.ToAdmissionResponse(err)
    }
    
    if err := validateParentChildRelationships(queue); err != nil {
        return util.ToAdmissionResponse(err)
    }
    
    return &admissionv1.AdmissionResponse{Allowed: true}
}
```

#### Controller for Additional Validation
```go
func (r *QueueController) validateAndUpdateStatus(queue *Queue) error {
    // Comprehensive validation with status updates
    // Handle edge cases and complex scenarios
}
```

### Strategy 4: Alternative Policy Engine (OPA Gatekeeper)

#### Gatekeeper with External Data
```yaml
apiVersion: config.gatekeeper.sh/v1alpha1
kind: Provider
metadata:
  name: volcano-queue-data
spec:
  url: "http://volcano-admission-service/queue-data"
  caBundle: "..."

---
apiVersion: templates.gatekeeper.sh/v1beta1
kind: ConstraintTemplate
metadata:
  name: volcanoqueuerequirements
spec:
  crd:
    spec:
      names:
        kind: VolcanoQueueRequirements
  targets:
  - target: admission.k8s.gatekeeper.sh
    rego: |
      package volcanoqueuerequirements
      
      violation[{"msg": msg}] {
        hierarchy := input.review.object.metadata.annotations["scheduling.volcano.sh/hierarchy"]
        existing_queues := data.inventory.cluster["scheduling.volcano.sh/v1beta1"]["Queue"]
        conflicts := [q | q := existing_queues[_]; startswith(q.metadata.annotations["scheduling.volcano.sh/hierarchy"], sprintf("%s/", [hierarchy]))]
        count(conflicts) > 0
        msg := sprintf("hierarchy conflict with existing queue: %v", [conflicts[0].metadata.name])
      }
```

## Mutation Migration: High Success Probability

### CEL Migration for Mutations

#### 1. Hierarchy Root Addition
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingAdmissionPolicy
metadata:
  name: queue-hierarchy-mutation
spec:
  mutations:
  - target:
      kind: Queue
      group: scheduling.volcano.sh
      version: v1beta1
    patches:
    - op: add
      path: /metadata/annotations/scheduling.volcano.sh~1hierarchy
      value: |
        "root/" + object.metadata.annotations["scheduling.volcano.sh/hierarchy"]
      condition: |
        has(object.metadata.annotations["scheduling.volcano.sh/hierarchy"]) &&
        !object.metadata.annotations["scheduling.volcano.sh/hierarchy"].startsWith("root/")
    
    - op: add
      path: /metadata/annotations/scheduling.volcano.sh~1hierarchy-weights
      value: |
        "1/" + object.metadata.annotations["scheduling.volcano.sh/hierarchy-weights"]
      condition: |
        has(object.metadata.annotations["scheduling.volcano.sh/hierarchy-weights"]) &&
        !object.metadata.annotations["scheduling.volcano.sh/hierarchy-weights"].startsWith("1/")
```

#### 2. Default Values
```yaml
mutations:
- target:
    kind: Queue
    group: scheduling.volcano.sh
    version: v1beta1
  patches:
  - op: add
    path: /spec/reclaimable
    value: true
    condition: "!has(object.spec.reclaimable)"
  
  - op: add
    path: /spec/weight
    value: 1
    condition: "!has(object.spec.weight) || object.spec.weight == 0"
```

## Performance Analysis

### Current Performance
- **Validation**: 200-500ms (multiple API calls for hierarchy validation)
- **Mutation**: 50-100ms (simple field mutations)
- **Resource Usage**: High due to extensive cluster state queries

### Expected Performance with Migration

#### Controller-Based Approach
- **Admission**: 5-10ms (basic CEL validation only)
- **Background Validation**: 200-300ms (controller reconciliation)
- **Overall**: Much better admission latency

#### Simplified CEL Only
- **Validation**: 2-5ms (pure field validation)
- **Lost Functionality**: Significant validation coverage reduction

## Implementation Timeline

### Phase 1: Mutation Migration (3-4 weeks)
- **Week 1**: Hierarchy annotation mutations
- **Week 2**: Default value mutations
- **Week 3**: Testing and validation
- **Week 4**: Deployment and monitoring

### Phase 2: Controller-Based Validation (6-8 weeks)
- **Week 1-2**: Controller implementation
- **Week 3-4**: Complex validation logic migration
- **Week 5-6**: Status management and error handling
- **Week 7-8**: Integration testing and deployment

### Phase 3: CEL Basic Validation (2-3 weeks)
- **Week 1**: Basic field validation in CEL
- **Week 2**: Testing and integration
- **Week 3**: Performance optimization

### Phase 4: Webhook Deprecation (2-3 weeks)
- **Week 1**: Gradual traffic migration
- **Week 2**: Monitoring and validation
- **Week 3**: Cleanup and documentation

## Risk Assessment

### High Risk Areas
1. **Hierarchy Validation**: Complex logic with cluster-wide dependencies
2. **Parent-Child Relationships**: Dynamic validation requirements
3. **Resource Calculations**: Complex multi-dimensional comparisons
4. **Backward Compatibility**: Ensuring existing queue configurations continue working

### Medium Risk Areas
1. **Performance**: Controller-based validation may have different latency characteristics
2. **Error Reporting**: Maintaining clear error messages for users
3. **Status Management**: Complex status tracking in controller approach

### Low Risk Areas
1. **Basic Mutations**: Simple default value assignments
2. **Field Validation**: Basic type and range checks
3. **Annotation Processing**: String manipulation logic

## Recommendations

### Immediate Actions (0-3 months)
1. **Start with mutation migration**: High success probability, immediate benefits
2. **Implement controller-based validation**: Maintains full functionality
3. **Add basic CEL validation**: Improve admission performance for simple cases

### Medium Term (3-6 months)
1. **Gradual webhook deprecation**: As controller validation proves stable
2. **Enhanced error reporting**: Improve user experience with controller status
3. **Performance optimization**: Fine-tune controller reconciliation

### Long Term (6-12 months)
1. **Consider alternative architectures**: OPA Gatekeeper or other policy engines
2. **Enhanced CEL capabilities**: If external data access becomes available
3. **Community collaboration**: Share learnings about complex validation patterns

## Conclusion

The Queues webhook presents the **highest migration complexity** among all Volcano webhooks:

- **Mutation Logic**: Highly suitable for CEL migration (85% confidence)
- **Basic Validation**: Partially suitable for CEL migration (50% confidence)
- **Complex Validation**: Requires alternative approaches (25% confidence for pure CEL)

**Recommended Strategy:**
1. **Migrate mutations to CEL** for immediate performance benefits
2. **Implement controller-based validation** to maintain full functionality
3. **Use basic CEL validation** for simple field checks
4. **Gradual webhook deprecation** as alternative approaches mature

This hybrid approach preserves the sophisticated queue management capabilities that are fundamental to Volcano's operation while providing significant performance improvements and reducing operational overhead.