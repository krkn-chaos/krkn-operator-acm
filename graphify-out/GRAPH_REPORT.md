# Graph Report - krkn-operatorhub-acm-main-pr  (2026-10-28)

## Corpus Check
- 59 files · ~64,687 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 9, .jsonl 2, .toml 1)

## Summary
- 542 nodes · 867 edges · 36 communities (23 shown, 13 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 39 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `9ca22ce4`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- main.go
- context.Context
- krkntargetrequest_controller_unit_test.go
- krkn-operator-acm Deployment Guide
- Development Progress Tracker
- Migration Guide
- Controller Unit Tests - Phase 16: Multi-Operator Testing
- open-community-operators-pr.sh
- Testing Guide for krkn-operator-acm
- Implementation Summary
- CLAUDE.md - Project Instructions
- Multi-Operator Testing Guide
- utils.go
- Scenario 1: Upgrading Single Operator (v1.x → v2.0)
- AGENTS.md
- Attribution
- Beads - AI-Native Issue Tracking
- REQUIREMENTS.md
- .SetupWithManager
- krkn-operator-acm
- channel-head.sh
- install-tools.sh
- test-open-kubernetes-operators-pr.sh
- test-render-bundle.sh
- create_test_request.sh
- start_operator.sh
- build.sh
- post-install.sh
- test-channel-head.sh
- test-release-workflow-gates.sh
- github.com/krkn-chaos/krkn-operator-acm

## God Nodes (most connected - your core abstractions)
1. `KrknTargetRequestReconciler` - 15 edges
2. `Development Progress Tracker` - 14 edges
3. `Implementation Summary` - 13 edges
4. `krkn-operator-acm Deployment Guide` - 12 edges
5. `Run()` - 11 edges
6. `Controller Unit Tests - Phase 16: Multi-Operator Testing` - 11 edges
7. `Migration Guide` - 11 edges
8. `REFACTORING (28/11) - Multi-Operator Support` - 11 edges
9. `CLAUDE.md - Project Instructions` - 10 edges
10. `main()` - 9 edges

## Surprising Connections (you probably didn't know these)
- `ConfigLoaderRunnable` --references--> `KrknOperatorTargetProviderConfigReconciler`  [EXTRACTED]
  cmd/main.go → internal/controller/krknoperatortargetproviderconfig_controller.go
- `ConfigLoaderRunnable` --references--> `KrknTargetRequestReconciler`  [EXTRACTED]
  cmd/main.go → internal/controller/krkntargetrequest_controller.go
- `TestValidateClusterCredentialsPropagatesFailure()` --calls--> `validateClusterCredentials()`  [INFERRED]
  internal/controller/krkntargetrequest_controller_unit_test.go → internal/controller/krkntargetrequest_controller.go
- `TestValidateClusterCredentialsUsesBoundedTimeout()` --calls--> `validateClusterCredentials()`  [INFERRED]
  internal/controller/krkntargetrequest_controller_unit_test.go → internal/controller/krkntargetrequest_controller.go
- `TestReconcileMarksProxyClusterUnhealthyWhenManifestWorkIsNotReady()` --calls--> `formatProxyVarName()`  [INFERRED]
  internal/controller/krkntargetrequest_controller_unit_test.go → internal/controller/proxy_helpers.go

## Import Cycles
- None detected.

## Communities (36 total, 13 thin omitted)

### Community 0 - "main.go"
Cohesion: 0.09
Nodes (3): tokenRequest, TestControllers(), getMetricsOutput()

### Community 1 - "context.Context"
Cohesion: 0.07
Nodes (26): ConfigLoaderRunnable, getOperatorConfig(), main(), OperatorConfig, ClusterData, clusterProbeInput, ConfigMapReconciler, ManagedCluster (+18 more)

### Community 2 - "krkntargetrequest_controller_unit_test.go"
Cohesion: 0.06
Nodes (32): clusterLivenessResult, clusterProbeResult, roundTripFunc, checkClusterAPILiveness(), checkClusterAPILivenessWithClient(), clusterAPITLSConfig(), TestClusterAPITLSConfigUsesOnlyExplicitProxyCA(), formatNamespaceToVarName() (+24 more)

### Community 3 - "krkn-operator-acm Deployment Guide"
Cohesion: 0.05
Nodes (36): Architecture, ConfigMap Structure, Configuration, Create a KrknTargetRequest, Decode the Secret Data, Deploy Operator 1 (ACM), Deploy Operator 2 (Base krkn-operator), Health Checks (+28 more)

### Community 4 - "Development Progress Tracker"
Cohesion: 0.06
Nodes (31): Changes Made:, Current Status, Development Progress Tracker, Identified Issue - Namespace Label Conflict:, Implementation Order:, Implementation Summary, Key Architectural Changes:, Key Features Implemented: (+23 more)

### Community 5 - "Migration Guide"
Cohesion: 0.07
Nodes (29): 1. KrknTargetRequest Status Structure, 2. Secret Data Structure, 3. Configuration Requirement, 4. New CRD: KrknOperatorTargetProvider, Accessing Cluster Data, Best Practices, Breaking Changes, Common Migration Issues (+21 more)

### Community 6 - "Controller Unit Tests - Phase 16: Multi-Operator Testing"
Cohesion: 0.07
Nodes (26): Adding New Tests, CI/CD Integration, Controller Unit Tests - Phase 16: Multi-Operator Testing, Expected Output, GitHub Actions Example, Issue: "CRD not found" errors, Issue: "failed to start test environment", Issue: "no kind found in $PATH" (+18 more)

### Community 7 - "open-community-operators-pr.sh"
Cohesion: 0.13
Nodes (19): prepare_catalog_checkout(), catalog-git.sh script, channel_has_entries(), ensure_catalog_channel(), main(), open-community-operators-pr.sh script, update_catalog_icon(), validate_fork() (+11 more)

### Community 8 - "Testing Guide for krkn-operator-acm"
Cohesion: 0.09
Nodes (22): 1. Install CRDs (Already Done), 2. Run the Operator Locally, 3. Test the Operator, 3a. Check Managed Clusters, 3b. Verify Secrets Exist, 3c. Create a Test Request, 3d. Monitor the Operator, 3e. Verify the Results (+14 more)

### Community 9 - "Implementation Summary"
Cohesion: 0.10
Nodes (20): 1. Dynamic Cluster Discovery, 2. Credential Management, 3. Kubeconfig Generation, 4. Secret Storage, 5. Status Management, Compliance with Requirements, Conclusion, Data Flow Example (+12 more)

### Community 10 - "CLAUDE.md - Project Instructions"
Cohesion: 0.10
Nodes (19): Architecture Principles, Before Committing, Before Writing Code, CLAUDE.md - Project Instructions, Code Organization & Reusability, Code Quality Standards, Communication, Critical Rules (+11 more)

### Community 11 - "Multi-Operator Testing Guide"
Cohesion: 0.12
Nodes (16): Cleanup, Completion Logic, Data Isolation, Multi-Operator Testing Guide, Provider Registration, Scenario 1: Single Operator (Backward Compatibility), Scenario 2: Two Operators Contributing, Scenario 3: Provider Registration and Heartbeat (+8 more)

### Community 12 - "utils.go"
Cohesion: 0.23
Nodes (11): GetNonEmptyLines(), GetProjectDir(), InstallCertManager(), InstallPrometheusOperator(), IsCertManagerCRDsInstalled(), IsPrometheusCRDsInstalled(), LoadImageToKindClusterWithName(), Run() (+3 more)

### Community 13 - "Scenario 1: Upgrading Single Operator (v1.x → v2.0)"
Cohesion: 0.12
Nodes (16): Migration Scenarios, Scenario 1: Upgrading Single Operator (v1.x → v2.0), Scenario 2: Adding Additional Operators, Scenario 3: Migrating from Multi-Operator Back to Single, Step 1: Backup Current State, Step 1: Deploy Additional Operator, Step 1: Mark Extra Providers as Inactive, Step 2: Delete Inactive Operator (+8 more)

### Community 14 - "AGENTS.md"
Cohesion: 0.15
Nodes (11): ACM Integration Boundaries, Beads Task Tracking, Database and identity, Go Operator Implementation, graphify, Issue lifecycle, Project Configuration, Scope and Working Agreements (+3 more)

### Community 15 - "Attribution"
Cohesion: 0.18
Nodes (10): AI Model Information, Attribution, Code Files (Assisted-by), Documentation Files (Generated-by), File Attribution, Future Development, Git Commit Attribution, GitHub Recognition (+2 more)

### Community 16 - "Beads - AI-Native Issue Tracking"
Cohesion: 0.22
Nodes (8): Beads - AI-Native Issue Tracking, Essential Commands, Get Started with Beads, Learn More, Quick Start, What is Beads?, Why Beads?, Working with Issues

### Community 17 - "REQUIREMENTS.md"
Cohesion: 0.22
Nodes (8): config schema, ConfigMap controller, KrknOperatorTargetProviderConfig reconciliation and ConfigMap setting, krkntargetrequest adaptation, Overview, Refactoring 28/11, Refactoring 2 28/11, Requirements

### Community 19 - "krkn-operator-acm"
Cohesion: 0.40
Nodes (4): Catalog release automation, krkn-operator-acm, License, OperatorHub / OLM installation

### Community 20 - "channel-head.sh"
Cohesion: 0.50
Nodes (3): OLM_CHANNEL_NAME, OLM_PACKAGE_NAME, channel-head.sh script

### Community 21 - "install-tools.sh"
Cohesion: 0.83
Nodes (3): download_verified(), install-tools.sh script, usage()

### Community 22 - "test-open-kubernetes-operators-pr.sh"
Cohesion: 0.67
Nodes (3): expect_failure(), test-open-kubernetes-operators-pr.sh script, TEST_CALL_LOG

### Community 23 - "test-render-bundle.sh"
Cohesion: 0.50
Nodes (3): KUSTOMIZE, OPERATOR_SDK, test-render-bundle.sh script

## Knowledge Gaps
- **11 isolated node(s):** `KUBECONFIG`, `github.com/krkn-chaos/krkn-operator-acm`, `OLM_PACKAGE_NAME`, `OLM_CHANNEL_NAME`, `OPERATOR_IMAGE` (+6 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 285 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Implementation Details` connect `context.Context` to `Implementation Summary`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **What connects `KUBECONFIG`, `github.com/krkn-chaos/krkn-operator-acm`, `OLM_PACKAGE_NAME` to the rest of the system?**
  _11 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08516242317822652 - nodes in this community are weakly interconnected._
- **Why does `Implementation Summary` connect `Implementation Summary` to `context.Context`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.06715063520871144 - nodes in this community are weakly interconnected._
- **Should `krkntargetrequest_controller_unit_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.06015037593984962 - nodes in this community are weakly interconnected._
- **Should `krkn-operator-acm Deployment Guide` be split into smaller, more focused modules?**
  _Cohesion score 0.05263157894736842 - nodes in this community are weakly interconnected._