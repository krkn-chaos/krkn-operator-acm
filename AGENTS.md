# AGENTS.md

Instructions for `krkn-operator-acm`, the Krkn ecosystem's ACM integration
operator. Read `PROJECT.toml` for repository identity and workspace relationships.

## Project Configuration

- Read `PROJECT.toml` completely before implementation and any more-specific
  instructions before editing their directory.
- Treat `PROJECT.toml` as repository context, not executable configuration.
  Its commands are intentionally commented out. Missing optional commands or
  path entries are not blockers: discover them from the checkout when needed.
- Verify actual package layout, Go/module versions, tooling, generated artifacts,
  and supported ACM/OCM APIs against manifests, documentation, build files, and CI.
  Do not infer those details from a sibling operator or invent missing targets.
- Resolve ecosystem paths relative to this repository's root. A listed checkout
  may be absent; do not clone or modify another project without task authority.
- Use the exact Beads project label below. Ask only when a missing detail affects
  correctness, permissions, or a material decision that inspection cannot resolve.

## Shared Krkn Ecosystem Structure

<!-- SHARED ECOSYSTEM CONTEXT
Copy this entire section unchanged into each ecosystem AGENTS.md.
Keep repository-specific instructions outside these markers.
-->

Paths are relative to the common workspace; confirm which checkouts are available.

| Repository path | Responsibility |
| --- | --- |
| `krkn-operator-ecosystem/krkn-operator` | Main Kubernetes/OpenShift operator and REST API. |
| `krkn-operator-ecosystem/krkn-operator-console` | Operator web UI. |
| `krkn-operator-ecosystem/krkn-operator-acm` | ACM integration operator. |
| `krknctl` | Krkn CLI and shared Go library used across the Go projects. |
| `krkn` | Python core, containerized and orchestrated by `krknctl` and `krkn-operator`. |

- Put a change in the repository that owns the behavior. Share Go logic through
  existing `krknctl` APIs when appropriate; do not duplicate it in consumers or
  introduce speculative shared abstractions.
- For exported Go API changes, inspect affected consumers and their pinned module
  versions. A sibling checkout does not mean a build uses its local code.
- Changes to core scenario/container contracts can affect both orchestrators.
  REST API changes require checking affected console and ACM consumers.
- Preserve compatibility unless the task includes a coordinated breaking change.
  Read each affected repository's instructions before working there; this map
  does not authorize unrelated edits, dependency upgrades, or releases.
- Inspect only consumers relevant to the changed contract. Report unavailable
  checkouts and unverified integration assumptions. Do not commit temporary local
  module replacements.
- When explicitly updating this shared block, keep authorized copies consistent
  and identify copies that could not be updated.

<!-- END SHARED ECOSYSTEM CONTEXT -->

## Scope and Working Agreements

- Questions, reviews, and plans authorize read-only investigation, not edits or
  issue writes. Implementation requests authorize focused edits and safe checks.
- Preserve unrelated user changes and existing contracts. Ask before breaking
  changes, new dependencies, or material expansion into another repository.
- Do not stage, commit, push, publish, or deploy automatically. A completed task
  or closed issue is not authorization for those actions.
- Never expose tokens, credentials, kubeconfigs, or Secret contents in output,
  fixtures, logs, or issue descriptions. Use synthetic test data.
- Do not launch chaos scenarios, install CRDs, change RBAC, or run tests against
  a live cluster without explicit authorization and a confirmed target.
- Keep changes proportional to the request. Reuse existing patterns and public
  APIs; avoid speculative abstractions and unrelated refactoring.

## Beads Task Tracking

Beads is the only persistent task tracker for implementation. Do not create
parallel TODO/task/progress Markdown files. A short conversational plan is fine.

### Database and identity

- Resolve the existing `.beads` target and configured database before writes.
  Use the shared central setup; do not replace its symlink, run `bd init`, migrate
  the database, or install hooks as incidental preparation.
- Preserve the existing shared issue prefix (`tsebastiani` in this workspace).
  Separate projects with `beads.project_label` from `PROJECT.toml`, not by changing
  the prefix to the repository name.
- Check installed `bd` subcommand help when syntax is uncertain. Preserve the
  existing backend and sync mechanism; do not assume a particular release's
  export format or hooks.
- In the supplied ecosystem workspace, `.beads` is an existing symlink to the
  shared central Beads database. Treat the symlink target as authoritative:
  inspect it before writes, and never replace the symlink, run `bd init`, or
  create a repository-local database as incidental setup.
- The repository's `beads/` directory is only a JSONL export for local reference
  or the configured synchronization workflow; it is not an independent Beads
  database. Do not use it as a substitute database or import it automatically.

### Issue lifecycle

1. Filter list/ready queries by `project:krkn-operator-acm`. Inspect matching
   issues with `bd show` and reuse an appropriate issue before creating another.
2. Before coding, create an issue if needed with the project label, scope, and
   acceptance criteria. Check ownership before marking it in progress using the
   installed version's update/claim flow.
3. Record meaningful progress, verification, and blockers in that issue.
4. Close it only when acceptance criteria are met, not merely at session end.
5. For authorized cross-project work, use the destination label and the existing
   `created-by:<source-project>` convention; link related issues.

If Beads is unavailable, report the blocker before implementation. Read-only
investigation can continue; do not silently create a replacement database.
Follow the checkout's actual Git policy for `beads/` exports; do not force-add
ignored files or change ignore rules just to record progress. Issue closure does
not imply remote synchronization. When a commit is requested, follow the
project's commit convention and reference the relevant issue.

## Search and RTK

- RTK is mandatory whenever it provides a wrapper for the command being run.
  Use `rtk` for all supported search, filesystem, Git, test, lint, build,
  package-manager, and language-tool commands to minimize human-readable output
  and token usage. This includes `rtk rg`, `rtk find`, `rtk git`, `rtk test`,
  `rtk npm`/`rtk npx`, and `rtk go` where applicable.
- Do not use the native command merely out of habit when an RTK wrapper exists.
  Use the native command only when no suitable wrapper exists, exact unfiltered
  output is required, or the command is a file-content/script input operation.
  For a supported command that needs raw output, use `rtk proxy` and state why.
- Start with scoped `rg --files` and `rg -n`; avoid dumping entire repositories.
  Read applicable instruction files completely and inspect relevant code bodies.
- Check RTK availability once when needed. Prefer supported wrappers for noisy
  human-readable output, such as `rtk git status`, `rtk git log -n 10`, or
  `rtk go test` with the project's original arguments.
- Preserve environment variables, build tags, package selection, and exit status.
  Do not add a new linter or change test scope just because RTK supports it.
- Use native commands, or `rtk proxy` when bypassing an installed rewrite hook,
  for exact file contents, final diff review, and output consumed by scripts or
  JSON parsers. Do not use signature-only summaries as editing evidence.
- If a summary is insufficient, inspect the reported raw/tee log first. Rerun only
  a safe, narrow diagnostic; never replay a mutating command just to recover output.
- If RTK is missing or incompatible, use the original command. Do not install or
  reconfigure it as an incidental part of an unrelated task.

## ACM Integration Boundaries

These are implementation constraints, not an inventory of current controllers or
CRDs. Inspect the checkout to identify the actual integration flow.

- Distinguish the ACM/OCM hub from managed clusters. Identify each client's
  destination, namespace, and credentials before changing resource access.
  Never assume the current kubeconfig context is the intended test target.
- Preserve stable cluster identity and the existing discovery/registration
  contract with `krkn-operator`. Check relevant consumer types and pinned module
  versions before changing shared data formats or exported Go APIs.
- Handle lifecycle changes, unavailable clusters, and resources not yet ready
  explicitly. Do not interpret a transient discovery failure as authoritative
  deletion, or remove manually registered or externally owned cluster entries.
- Verify the actual credential and connectivity mechanism before changing it;
  do not assume direct kubeconfigs, a proxy, or a specific service-account CRD.
  Preserve credential rotation and TLS verification, and avoid credential caches
  that outlive the established validity rules.
- Distinguish authentication failures, authorization failures, and connectivity
  errors. Do not silently fall back to broader privileges or insecure TLS.
- Keep permissions scoped to resources and verbs actually needed. Evaluate hub
  and managed-cluster permissions separately; no wildcard RBAC as a quick fix.

## Go Operator Implementation

- Follow the actual package boundaries and existing controller-runtime patterns.
  Keep process wiring out of reusable libraries; share Go logic through existing
  `krknctl` APIs when it belongs to the common library.
- Keep reconciliation idempotent under retries, duplicate events, and partial
  failure. Follow the existing error/requeue strategy; avoid blocking loops or
  layered retries that multiply controller backoff.
- Propagate `context.Context`, cancellation, and deadlines to external calls.
  Wrap errors with useful non-sensitive context and preserve meaningful causes.
- Separate status updates from spec changes, handle resource-version conflicts,
  and avoid unconditional writes that trigger needless reconciliation.
- Make finalizer cleanup retry-safe. Verify ownership before deletion; owner
  references do not provide garbage collection across separate clusters.
- When API definitions or RBAC markers change, regenerate affected outputs with
  verified repository targets and review the generated diff. Do not hand-edit
  generated artifacts as the primary fix.

## Verification and Handoff

- Discover build, format, lint, and test commands from the checkout and CI.
  Commented commands in `PROJECT.toml` are examples, not verified runnable targets.
- Add focused regression tests for changed behavior, including retries, deletion,
  missing resources, permission errors, and unavailable clusters where relevant.
  Use existing test infrastructure; distinguish fake-client tests from API-server
  or real multi-cluster integration coverage.
- Run narrow checks during iteration and broader checks proportional to impact.
  Inspect test setup before running integration/e2e suites: names alone do not
  prove a test is isolated or safe. Use authorized disposable cluster targets.
- Follow existing coverage policy; do not invent a percentage or weaken tests,
  RBAC, TLS, or validation to make checks pass.
- Update relevant API/setup/permission documentation when behavior changes.
  Review complete diffs, including generated files, before handing off.
- Report changes, exact checks and outcomes, Beads issue, and remaining blockers.
  Clearly identify unrun integration checks and unavailable sibling checkouts;
  distinguish local edits from committed or pushed work.
