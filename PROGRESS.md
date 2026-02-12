
# Development Progress Tracker

<!-- Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

## Project: krkn-operator-acm

Integration operator for RedHat ACM and krkn-operator

---

## Phase 1: Project Setup & CRD Definition

- [x] Initialize Go operator project structure
- [x] Define `KrknTargetRequest` CRD with fields:
  - [x] UUID (string)
  - [x] targetData (JSON field)
  - [x] status (string - pending/completed)
- [x] Generate CRD manifests
- [x] Set up operator controller scaffolding

## Phase 2: Watch & React to KrknTargetRequest

- [x] Implement controller to watch `KrknTargetRequest` CRs
- [x] Add reconciliation logic for pending requests
- [x] Implement status update mechanism

## Phase 3: ACM Integration - Managed Cluster Discovery

- [x] Implement API client for ACM
- [x] Query `/apis/cluster.open-cluster-management.io/v1/managedclusters`
- [x] Parse response and extract:
  - [x] Cluster names (`.items[].name`)
  - [x] Cluster API URLs (`.items[].spec.managedClusterClientConfigs[].url`)
  - [x] Cluster CA Bundle (`.items[].spec.managedClusterClientConfigs[].caBundle`)

## Phase 4: Secret Retrieval from Managed Cluster Namespaces

- [x] For each managed cluster:
  - [x] Access namespace matching cluster name
  - [x] Retrieve `application-manager` secret
  - [x] Decode base64 fields:
    - [x] `ca.crt` (CA certificate)
    - [x] `token` (API authentication token)

## Phase 5: Kubeconfig Generation

- [x] Implement kubeconfig builder for each cluster
- [x] Populate kubeconfig with:
  - [x] Cluster API URL
  - [x] CA certificate
  - [x] Token authentication
- [x] Validate generated kubeconfig format

## Phase 6: Secret Creation with Target Data

- [x] Create secret named with request UUID
- [x] Build `.data.managed-clusters` JSON map:
  - [x] Key: cluster name
  - [x] Value object containing:
    - [x] cluster-name
    - [x] cluster-api
    - [x] kubeconfig (base64 encoded)
- [x] Apply secret to cluster

## Phase 7: Update KrknTargetRequest Status

- [x] Update `KrknTargetRequest` status to "Completed"
- [x] Populate `targetData` field with list of:
  - [x] cluster-name
  - [x] cluster-api-url

## Phase 8: Testing & Validation

- [ ] Unit tests for:
  - [ ] ACM API parsing
  - [ ] Kubeconfig generation
  - [ ] Secret creation logic
- [ ] Integration tests:
  - [ ] End-to-end workflow test
  - [ ] Error handling scenarios
- [ ] Manual testing with actual ACM environment

## Phase 9: Documentation & Deployment

- [x] Write operator deployment manifests (generated via kubebuilder)
- [ ] Create README with:
  - [ ] Installation instructions
  - [ ] Configuration guide
  - [ ] Usage examples
- [x] Document RBAC requirements (generated automatically)
- [x] Create example `KrknTargetRequest` CR

---

## Current Status

**Phase:** Phase 7 - Core Implementation Complete
**Last Updated:** 2025-11-27
**Blocker:** None

## Implementation Summary

All core phases (1-7) have been completed:
- ✅ CRD Definition: `api/v1alpha1/krkntargetrequest_types.go`
- ✅ Controller Implementation: `internal/controller/krkntargetrequest_controller.go`
- ✅ Main Entrypoint: `cmd/main.go`
- ✅ Generated CRD: `config/crd/bases/krkn.krkn-chaos.dev_krkntargetrequests.yaml`
- ✅ Sample CR: `config/samples/krkn_v1alpha1_krkntargetrequest.yaml`

### Key Features Implemented:
1. **Dynamic ACM Integration**: Uses unstructured client to query ManagedCluster resources
2. **Secret Retrieval**: Fetches `application-manager` secrets from cluster namespaces
3. **Kubeconfig Generation**: Creates valid kubeconfig files for each managed cluster
4. **Secret Storage**: Creates a secret with UUID as name containing cluster data
5. **Status Management**: Updates KrknTargetRequest status to "Completed" with targetData

## Notes

- Reference sample data: `misc/managed-cluster.json`
- CRD typo in requirements: `caBunble` should likely be `caBundle`

---

# REFACTORING (28/11) - Multi-Operator Support

## Phase 10: KrknOperatorTargetProvider CRD

- [x] Create `KrknOperatorTargetProvider` CRD with fields:
  - [x] `operator-name` (string) - unique identifier for the operator
  - [x] `timestamp` (timestamp) - last heartbeat/update time
- [x] Generate CRD manifests for KrknOperatorTargetProvider
- [x] Add RBAC permissions for the new CRD

## Phase 11: Operator Registration & Configuration

- [x] Create ConfigMap for operator configuration:
  - [x] Define ConfigMap structure with `operator-name` field
  - [x] Create sample ConfigMap in `config/samples/`
  - [x] Document ConfigMap fields for future expansion
- [x] Implement ConfigMap reading logic:
  - [x] Read operator-name from ConfigMap at startup
  - [x] Make ConfigMap accessible throughout reconcile loop
  - [x] Add error handling for missing ConfigMap
- [x] Implement operator registration at boot:
  - [x] Check if KrknOperatorTargetProvider CR exists with operator-name
  - [x] Create new CR if not exists
  - [x] Update timestamp if CR already exists
  - [x] Add reconcile logic for keeping registration up-to-date

## Phase 12: Timestamp Management

- [x] Add timestamp fields to KrknTargetRequest:
  - [x] `created` (timestamp) - when CR was created
  - [x] `completed` (timestamp) - when processing completed
- [x] Update CRD with new timestamp fields
- [x] Implement timestamp setting logic:
  - [x] Set `created` when status is initialized to "pending"
  - [x] Set `completed` when status changes to "Completed"

## Phase 13: Multi-Operator TargetData Structure

- [x] Refactor `KrknTargetRequestStatus.TargetData`:
  - [x] Change from `[]ClusterTarget` to `map[string][]ClusterTarget`
  - [x] Key: operator-name
  - [x] Value: array of ClusterTarget for that operator
- [x] Update controller logic:
  - [x] Read operator-name from ConfigMap
  - [x] Set targetData using operator-name as key
  - [x] Handle merging with existing targetData from other operators
- [x] Update CRD manifests with new structure
- [x] Update sample CRs to reflect new structure

## Phase 14: Secret Data Structure Refactoring

- [x] Update Secret data structure:
  - [x] Change `.data.managed-clusters` to support multi-operator format
  - [x] Structure: `map[operator-name]map[cluster-name]ClusterData`
  - [x] Backward compatibility with old format (auto-migration)
- [x] Update secret creation logic:
  - [x] Read existing secret if present
  - [x] Merge new operator data with existing data
  - [x] Preserve data from other operators
- [x] Update secret reading/verification logic

## Phase 15: Multi-Operator Completion Logic

- [x] Implement provider counting:
  - [x] List all KrknOperatorTargetProvider CRs in reconcile loop
  - [x] Count active providers
  - [x] Log provider count for debugging
- [x] Update completion logic:
  - [x] Check if number of keys in TargetData map equals provider count
  - [x] Only set status to "Completed" when all providers have contributed
  - [x] Handle edge cases (providers disappearing, etc.)

## Phase 16: Testing Multi-Operator Scenario

- [x] Create test setup:
  - [x] Multiple ConfigMaps with different operator names
  - [x] Multiple operator instances
  - [x] Single KrknTargetRequest
- [x] Test scenarios:
  - [x] Single operator (backward compatibility)
  - [x] Two operators contributing to same request
  - [x] Provider registration and heartbeat
  - [x] Completion logic with multiple providers
- [x] Validate data integrity:
  - [x] Each operator's data isolated correctly
  - [x] Secret contains all operator data
  - [x] Status reflects all contributions

## Phase 17: Documentation Update

- [x] Update README with:
  - [x] Multi-operator architecture explanation
  - [x] ConfigMap configuration guide
  - [x] Provider registration mechanism
- [x] Update DEPLOYMENT.md with:
  - [x] ConfigMap deployment steps
  - [x] Multi-operator deployment example
- [x] Create migration guide:
  - [x] How to migrate from single to multi-operator
  - [x] Breaking changes documentation

---

## Refactoring Status

**Phase:** Phase 17 Complete - Multi-Operator Support Implementation Complete
**Last Updated:** 2025-12-01
**Priority:** High - Multi-operator support is critical for scalability

### Key Architectural Changes:

1. **New CRD**: `KrknOperatorTargetProvider` for operator registration
2. **Configuration**: ConfigMap-based operator configuration
3. **Data Structure**: Map-based TargetData to support multiple operators
4. **Completion Logic**: Dynamic completion based on registered provider count
5. **Timestamps**: Track CR lifecycle with created/completed timestamps
6. **Active Providers**: Only active providers contribute to completion logic
7. **Namespace Scoping**: All resources scoped to operator namespace
8. **Error Handling**: Conflict error handling with automatic retry

### Implementation Order:

1. ✅ Create KrknOperatorTargetProvider CRD (Phase 10)
2. ✅ Add ConfigMap support (Phase 11)
3. ✅ Add timestamps to KrknTargetRequest (Phase 12)
4. ✅ Refactor TargetData structure (Phase 13)
5. ✅ Update Secret structure (Phase 14)
6. ✅ Implement new completion logic (Phase 15)
7. ✅ Test multi-operator scenarios (Phase 16)
8. ✅ Update documentation (Phase 17)

### Refactoring 2 Completed (2025-12-01):

- ✅ Added `active` bool field to KrknOperatorTargetProvider CRD
- ✅ Count only active providers in reconcile loop
- ✅ Added operator-namespace to ConfigMap
- ✅ Created OperatorConfig object with getOperatorConfig method
- ✅ Removed getOperatorName method (replaced with config object)
- ✅ Fixed deprecated Requeue usage (line 133)
- ✅ ConfigMap is now single source of configuration
- ✅ Added conflict error handling throughout reconcile loop
- ✅ All resources (KrknTargetRequest, KrknOperatorTargetProvider, Secrets, ConfigMaps) scoped to operator namespace

---

## Phase 18: Namespace Configuration Fix (2026-01-08)

**Issue Found:** Operator was failing to start because ConfigMap lookup was using incorrect namespace `krkn-operator-acm-system` instead of the standard `krkn-operator-system`.

### Changes Made:

- [x] Fixed namespace references in all documentation files:
  - [x] DEPLOYMENT.md - Updated all namespace references from `krkn-operator-acm-system` to `krkn-operator-system`
  - [x] MIGRATION.md - Fixed ConfigMap and provider registration examples
  - [x] TESTING.md - Updated testing commands and kubectl examples
  - [x] test-multi-operator/README.md - Fixed all test scenarios and cleanup commands
- [x] Verified default namespace in cmd/main.go:382 is correct (`krkn-operator-system`)
- [x] Updated sample ConfigMap in config/samples/operator-config.yaml to use correct namespace

### Identified Issue - Namespace Label Conflict:

**Problem:** Both `krkn-operator` and `krkn-operator-acm` operators:
- Share the same namespace (`krkn-operator-system`)
- Define the namespace with different labels:
  - `krkn-operator`: `app.kubernetes.io/name: krkn-operator`
  - `krkn-operator-acm`: `app.kubernetes.io/name: krkn-operator-acm`
- When one operator is deployed, it updates the namespace labels
- This may cause resource cleanup issues if labels are used for selection

**Status:** ⚠️ POTENTIAL ISSUE - Needs investigation
**Next Steps:**
- [ ] Test deploying both operators simultaneously
- [ ] Verify resources don't get deleted when the other operator is deployed
- [ ] Consider making namespace definition optional or using separate namespaces
- [ ] Document co-deployment requirements if needed