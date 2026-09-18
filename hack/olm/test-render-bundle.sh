#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/krkn-acm-render-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT

export KUSTOMIZE=${KUSTOMIZE:-kustomize}
export OPERATOR_SDK=${OPERATOR_SDK:-operator-sdk}

sentinel_dir="$test_root/existing"
mkdir -p "$sentinel_dir"
printf 'preserve me\n' > "$sentinel_dir/sentinel"
if "$repo_root/hack/olm/render-bundle.sh" invalid-version "$sentinel_dir" >/dev/null 2>&1; then
  echo "renderer accepted an invalid version" >&2
  exit 1
fi
if "$repo_root/hack/olm/render-bundle.sh" 1.1.0-beta.1 "$sentinel_dir" >/dev/null 2>&1; then
  echo "renderer accepted an existing output directory" >&2
  exit 1
fi
grep -Fxq 'preserve me' "$sentinel_dir/sentinel"

output_dir="$test_root/bundle"
"$repo_root/hack/olm/render-bundle.sh" 1.1.0-beta.1 "$output_dir"
test -s "$output_dir/bundle.Dockerfile"
test -s "$output_dir/manifests/krkn-operator-acm.clusterserviceversion.yaml"
while IFS= read -r yaml_file; do
  test "$(head -n 1 "$yaml_file")" = "---"
  if awk 'length($0) > 180 { exit 1 }' "$yaml_file"; then
    :
  else
    echo "YAML line exceeds 180 characters: $yaml_file" >&2
    exit 1
  fi
done < <(find "$output_dir" -type f -name '*.yaml' -print)

echo "OLM renderer checks passed"
