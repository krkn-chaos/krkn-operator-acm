#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
script=(bash "$script_dir/open-kubernetes-operators-pr.sh")
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/test-open-kubernetes-operators-pr.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT

expect_failure() {
  local expected=$1
  shift
  local output
  if output=$("$@" 2>&1); then
    echo "expected command to fail: $*" >&2
    exit 1
  fi
  grep -Fq "$expected" <<<"$output" || {
    echo "expected '$expected' in command output, got:" >&2
    echo "$output" >&2
    exit 1
  }
}

expect_failure "invalid release version" "${script[@]}" invalid /missing

mkdir -p "$test_dir/manifests" "$test_dir/metadata"
printf '%s\n' '---' >"$test_dir/metadata/annotations.yaml"
expect_failure "does not contain a ClusterServiceVersion" \
  "${script[@]}" 1.0.0 "$test_dir"

printf '%s\n' '---' 'kind: ClusterServiceVersion' \
  >"$test_dir/manifests/operator.clusterserviceversion.yaml"
expect_failure "KUBERNETES_OPERATORS_FORK must be configured" \
  "${script[@]}" 1.0.0 "$test_dir"

echo "Kubernetes OperatorHub submission validation tests passed"
