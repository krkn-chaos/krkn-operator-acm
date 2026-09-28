# krkn-operator-acm

![test](https://github.com/krkn-chaos/krkn-operator-acm/actions/workflows/test.yml/badge.svg)
![pr-checks](https://github.com/krkn-chaos/krkn-operator-acm/actions/workflows/pr-checks.yml/badge.svg)
![coverage](https://krkn-chaos.github.io/krkn-lib-docs/coverage_badge_krkn-operator-acm.svg)


**Open Cluster Management integration for [Krkn Operator](https://github.com/krkn-chaos/krkn-operator).**

Krkn Operator ACM automatically discovers and configures access to clusters managed by Open Cluster Management (OCM) and Red Hat Advanced Cluster Management (ACM), enabling centralized chaos engineering across multi-cluster environments.

📖 **[Official Krkn Operator Documentation](https://krkn-chaos.gateway.scarf.sh/krkn-operator/docs?source=github-acm)**

## OperatorHub / OLM installation

The ACM integration is published as the `krkn-operator-acm` package in the
Krkn catalog. It requires the `krkn-operator` package, which OLM resolves when
both packages are available in the same catalog.

Install the packages through OperatorHub using these channels:

- `krkn-operator`: `stable-kubernetes`/`stable-ocp` for `1.0.x`, or versioned
  channels such as `stable-kubernetes-1.1`/`stable-ocp-1.1` for later lines;
- `krkn-operator-acm`: `stable-acm` for `1.0.x`, or a versioned channel such as
  `stable-acm-1.1` for later release lines.

Install the core package first, or select the ACM package and let OLM resolve
its `krkn-operator` dependency. The ACM operator supports its own namespace
and must be installed in the same namespace used for the shared Krkn custom
resources. The ACM bundle does not create an Ingress, Route, or Gateway.

For upstream OCM, install the Managed ServiceAccount add-on on the OCM hub
before enabling the ACM integration. Red Hat ACM and upstream OCM compatibility
is documented in the [compatibility matrix](https://krkn-chaos.dev/docs/krkn-operator/compatibility/).

## Catalog release automation

The release workflow automatically submits the rendered OLM bundle to the
`community-operators-prod` catalog after a version tag is published. Configure
these GitHub Actions organization-level settings before using the workflow:

- `COMMUNITY_OPERATORS_FORK`: repository variable containing the fork in the
  `<owner>/community-operators-prod` format;
- `COMMUNITY_OPERATORS_TOKEN`: secret containing a token that can push to the
  configured fork and open pull requests against the upstream catalog.

The token value must only be stored as a GitHub Actions secret. The workflow
selects the channel from the release line, creates a missing versioned channel,
and calculates `replaces` only from that channel. The first bundle in a new
channel has no `replaces`; later bundles write the matching predecessor. The
workflow also propagates the bundle icon to the catalog package metadata and
creates or updates the catalog pull request.

## License

Licensed under the [Apache License 2.0](LICENSE).
