# Controller Unit Tests - Phase 16: Multi-Operator Testing

This document describes the unit tests for Phase 16 of the krkn-operator-acm project.

## Test Framework

- **Framework**: Ginkgo (BDD-style testing) + Gomega (assertions)
- **Environment**: envtest (Kubernetes API server for testing)
- **Compatible with**: Kind, OpenShift CI, GitHub Actions

## Test Scenarios

### Scenario 1: Single Operator (Backward Compatibility)
**File**: `krkntargetrequest_controller_test.go`
**Test**: "should complete request when single active provider contributes"

**What it tests**:
- Single active provider can complete a request
- Status initializes to "pending"
- Status transitions to "Completed" when operator contributes
- TargetData contains single operator's data

**How it works**:
1. Creates one active KrknOperatorTargetProvider
2. Creates a KrknTargetRequest
3. Simulates operator contribution to TargetData
4. Verifies request completes (1/1 active providers contributed)

### Scenario 2: Two Operators Contributing
**Test**: "should remain pending until both operators contribute"

**What it tests**:
- Multiple operators can contribute to same request
- Request stays "pending" until all active operators contribute
- TargetData contains data from all operators
- Completion logic counts contributors correctly

**How it works**:
1. Creates two active KrknOperatorTargetProviders
2. Creates a KrknTargetRequest
3. Adds first operator's contribution → stays "pending" (1/2)
4. Adds second operator's contribution → transitions to "Completed" (2/2)
5. Verifies TargetData has both operators' data

### Scenario 3: Active vs Inactive Providers
**Test**: "should only count active providers for completion"

**What it tests**:
- Only active providers are counted for completion
- Inactive providers don't block request completion
- Active field is properly evaluated

**How it works**:
1. Creates one active and one inactive provider
2. Creates a KrknTargetRequest
3. Adds only active operator's contribution
4. Verifies request completes immediately (1/1 active providers)
5. Confirms inactive provider's data is not required

### Scenario 4: Provider Registration
**Tests**:
- "should have active field defaulting to true"
- "should allow creating inactive providers"

**What it tests**:
- Active field can be set to true
- Active field can be set to false
- Provider creation works correctly

### Scenario 5: Timestamp Management
**Test**: "should set created and completed timestamps"

**What it tests**:
- `created` timestamp is set when request is initialized
- `completed` timestamp is set when request completes
- Completed time is after created time

**How it works**:
1. Creates a request and verifies `created` timestamp
2. Simulates operator contribution
3. Verifies `completed` timestamp is set
4. Confirms completed > created

### Scenario 6: Conflict Error Handling
**Test**: "should handle conflict errors gracefully"

**What it tests**:
- Reconcile loop handles conflict errors without failing
- Multiple reconcile calls don't cause errors
- Request reaches stable state despite conflicts

**How it works**:
1. Creates a request
2. Calls reconcile multiple times (simulating concurrent updates)
3. Verifies no errors are returned
4. Confirms request reaches stable state

## Running the Tests

### Locally (without Kind)
```bash
# Setup envtest binaries
make setup-envtest

# Run all tests
make test

# Run with verbose output
make test ARGS="-v"
```

### With Kind
```bash
# Start Kind cluster
kind create cluster --name krkn-test

# Run tests (envtest still used, Kind not directly used by unit tests)
make test

# Cleanup
kind delete cluster --name krkn-test
```

### Run Specific Tests
```bash
# Run only Phase 16 tests
go test -v ./internal/controller -ginkgo.focus="Phase 16"

# Run specific scenario
go test -v ./internal/controller -ginkgo.focus="Single Operator"

# Run with coverage
go test -v ./internal/controller -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Structure

```
internal/controller/
├── suite_test.go                           # Test suite setup (BeforeSuite/AfterSuite)
├── krkntargetrequest_controller_test.go    # Main controller tests (Phase 16)
└── krknoperatortargetprovider_controller_test.go  # Provider controller tests
```

## CI/CD Integration

The tests are designed to run in CI pipelines:

### GitHub Actions Example
```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - name: Run tests
        run: make test
```

### Kind-based Integration Test Example
```yaml
- name: Create Kind cluster
  run: kind create cluster

- name: Load operator image
  run: |
    make docker-build
    kind load docker-image controller:latest

- name: Deploy operator
  run: make deploy

- name: Run integration tests
  run: go test -v ./test/e2e/...
```

## Expected Output

Successful test run:
```
Running Suite: Controller Suite - /path/to/krkn-operator-acm/internal/controller
================================================================================================

• [SLOW TEST] in Spec Setup (BeforeSuite) - 2.347s
  bootstrapping test environment

Running 8 specs in parallel across 4 processes

••••••••

Ran 8 of 8 Specs in 5.234 seconds
SUCCESS! -- 8 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS

coverage: 75.3% of statements
ok      github.com/krkn-chaos/krkn-operator-acm/internal/controller    5.567s  coverage: 75.3% of statements
```

## Troubleshooting

### Issue: "failed to start test environment"
**Solution**: Run `make setup-envtest` to install required binaries

### Issue: "no kind found in $PATH"
**Note**: Unit tests use envtest, not Kind. Kind is only needed for e2e tests.

### Issue: Tests hang or timeout
**Solution**:
- Check if envtest binaries are corrupted: `rm -rf bin/k8s && make setup-envtest`
- Increase timeout values in test if legitimate slow execution

### Issue: "CRD not found" errors
**Solution**: Ensure CRDs are generated: `make manifests`

## Test Coverage Goals

- **Phase 16 Multi-Operator**: Target 90%+ coverage
- **Overall Controller**: Target 75%+ coverage
- **Edge Cases**: All error paths should be tested

## Adding New Tests

Template for new test scenario:
```go
Describe("Scenario X: Description", func() {
    It("should do something", func() {
        By("Step 1: Setup")
        // Setup code

        By("Step 2: Action")
        // Trigger action

        By("Step 3: Verification")
        Eventually(func() bool {
            // Verification logic
        }, timeout, interval).Should(BeTrue())
    })
})
```

## Phase 16 Checklist

- [x] Scenario 1: Single operator backward compatibility
- [x] Scenario 2: Two operators contributing
- [x] Scenario 3: Active vs inactive providers
- [x] Scenario 4: Provider registration
- [x] Scenario 5: Timestamp management
- [x] Scenario 6: Conflict error handling
- [ ] Secret data structure validation (future)
- [ ] Namespace scoping tests (future)
- [ ] Provider heartbeat tests (future)