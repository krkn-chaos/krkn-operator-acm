# Multi-Operator Testing Guide

This directory contains resources for testing the multi-operator scenario (Phase 16).

## Test Scenarios

### Scenario 1: Single Operator (Backward Compatibility)
Tests that a single operator instance works correctly.

**Setup:**
1. Deploy operator with default ConfigMap (`krkn-operator-acm`)
2. Create a KrknTargetRequest
3. Verify it completes when the single operator contributes

**Expected Result:**
- KrknTargetRequest transitions to "Completed"
- Status.TargetData contains one key: "krkn-operator-acm"
- Secret contains data from one operator

**Test Command:**
```bash
./create_test_request.sh single-operator-test
```

### Scenario 2: Two Operators Contributing
Tests that multiple operators can contribute to the same request.

**Setup:**
1. Deploy first operator with ConfigMap (`krkn-operator-acm`)
2. Manually create second provider registration
3. Create a KrknTargetRequest
4. Manually update the request with second operator's data

**Expected Result:**
- KrknTargetRequest remains "pending" after first contribution
- KrknTargetRequest transitions to "Completed" after second contribution
- Status.TargetData contains two keys
- Secret contains data from both operators

**Note:** Since we only have one actual operator instance, we'll simulate the second operator by:
1. Creating a second KrknOperatorTargetProvider CR manually
2. Manually updating the KrknTargetRequest to add second operator's data

### Scenario 3: Provider Registration and Heartbeat
Tests that providers register correctly and update timestamps.

**Setup:**
1. Delete existing provider
2. Restart operator
3. Verify provider is recreated with active: true

**Expected Result:**
- Provider CR created automatically
- Timestamp updated on creation
- Active field set to true

### Scenario 4: Active vs Inactive Providers
Tests that only active providers are counted for completion.

**Setup:**
1. Create two providers: one active, one inactive
2. Create a KrknTargetRequest
3. Have only the active operator contribute

**Expected Result:**
- Request completes when active operator contributes (not waiting for inactive)
- Logs show "active-providers": 1, "total-providers": 2

## Testing Steps

### Test 1: Single Operator (Current Working State)
```bash
# This should work now
./create_test_request.sh test-single

# Check the request
kubectl get ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-single

# Should show status: Completed
kubectl get ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-single -o jsonpath='{.items[0].status.status}'
```

### Test 2: Simulate Two Operators
```bash
# Create a second provider manually
kubectl apply -f - <<EOF
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknOperatorTargetProvider
metadata:
  name: krkn-operator-test2
  namespace: krkn-operator-acm-system
spec:
  operator-name: "krkn-operator-test2"
  active: true
status:
  timestamp: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
EOF

# Now create a request - it should stay pending because we need 2 contributors
./create_test_request.sh test-multi

# Check status - should be "pending" with 1 contributor
kubectl get ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-multi -o yaml

# Manually add second operator's data to simulate second operator contribution
# This demonstrates the data structure
kubectl patch ktr request-test-multi -n krkn-operator-acm-system --type=merge -p '{
  "status": {
    "targetData": {
      "krkn-operator-test2": [
        {
          "cluster-name": "simulated-cluster",
          "cluster-api-url": "https://api.simulated.example.com:6443"
        }
      ]
    }
  }
}'

# Also update the secret to add second operator's data
# Get existing secret, add new operator data, update it
```

### Test 3: Active/Inactive Providers
```bash
# Set second provider to inactive
kubectl patch krknoperatortargetprovider krkn-operator-test2 -n krkn-operator-acm-system --type=merge -p '{"spec":{"active":false}}'

# Create new request - should complete with just one active provider
./create_test_request.sh test-active-only

# Check logs - should show active-providers: 1, total-providers: 2
# Request should complete immediately

# Cleanup - set back to active
kubectl patch krknoperatortargetprovider krkn-operator-test2 -n krkn-operator-acm-system --type=merge -p '{"spec":{"active":true}}'
```

## Validation Checklist

### Data Isolation
- [ ] Each operator's data in Status.TargetData is under its own key
- [ ] No operator overwrites another operator's data
- [ ] Secret data is properly namespaced by operator-name

### Completion Logic
- [ ] Single operator: completes when 1/1 contributes
- [ ] Two operators: stays pending at 1/2, completes at 2/2
- [ ] Inactive providers are not counted
- [ ] Active provider count is calculated correctly

### Provider Registration
- [ ] Provider auto-registers on operator boot
- [ ] Timestamp updates on registration
- [ ] Active field defaults to true
- [ ] Multiple providers can coexist

### Secret Data Structure
- [ ] Secret format: `map[operator-name]map[cluster-name]ClusterData`
- [ ] Multiple operators' data merged correctly
- [ ] No data loss when operators update secret
- [ ] Backward compatibility with old format (if applicable)

## Cleanup
```bash
# Remove test provider
kubectl delete krknoperatortargetprovider krkn-operator-test2 -n krkn-operator-acm-system

# Remove test requests
kubectl delete ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-single
kubectl delete ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-multi
kubectl delete ktr -n krkn-operator-acm-system -l krkn.krkn-chaos.dev/uuid=test-active-only

# Remove test secrets
kubectl delete secret test-single -n krkn-operator-acm-system
kubectl delete secret test-multi -n krkn-operator-acm-system
kubectl delete secret test-active-only -n krkn-operator-acm-system
```