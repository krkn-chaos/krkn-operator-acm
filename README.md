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

- `krkn-operator`: `stable-kubernetes` on Kubernetes or `stable-ocp` on OpenShift;
- `krkn-operator-acm`: `stable-acm`.

Install the core package first, or select the ACM package and let OLM resolve
its `krkn-operator` dependency. The ACM operator supports its own namespace
and must be installed in the same namespace used for the shared Krkn custom
resources. The ACM bundle does not create an Ingress, Route, or Gateway.

For upstream OCM, install the Managed ServiceAccount add-on on the OCM hub
before enabling the ACM integration. Red Hat ACM and upstream OCM compatibility
is documented in the [compatibility matrix](https://krkn-chaos.dev/docs/krkn-operator/compatibility/).

## License

Licensed under the [Apache License 2.0](LICENSE).
