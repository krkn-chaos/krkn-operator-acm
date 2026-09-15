#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <version> <output-dir>" >&2
  exit 2
}

[[ $# -eq 2 ]] || usage
version=$1
output_dir=$2

[[ -n "$output_dir" && "$output_dir" != "/" && "$output_dir" != "." && "$output_dir" != ".." ]] || {
  echo "output-dir must be a non-protected path" >&2
  exit 2
}

command -v yq >/dev/null || { echo "yq is required" >&2; exit 1; }

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
operator_sdk=${OPERATOR_SDK:-operator-sdk}
if command -v "$operator_sdk" >/dev/null 2>&1; then
  operator_sdk=$(command -v "$operator_sdk")
else
  operator_sdk="$repo_root/bin/operator-sdk"
fi
[[ -x "$operator_sdk" ]] || { echo "operator-sdk is required" >&2; exit 1; }
kustomize=${KUSTOMIZE:-$repo_root/bin/kustomize-v5.6.0}
[[ -x "$kustomize" ]] || { echo "kustomize is required" >&2; exit 1; }

output_parent=$(dirname "$output_dir")
mkdir -p "$output_parent"
output_dir="$(cd "$output_parent" && pwd)/$(basename "$output_dir")"
[[ "$output_dir" != "$repo_root" ]] || { echo "refusing to use repository root as output" >&2; exit 2; }

operator_image=${OPERATOR_IMAGE:-quay.io/krkn-chaos/krkn-operator-acm:${version}}
min_kube_version=${MIN_KUBE_VERSION:-1.19.0}
channel=${CHANNEL:-stable-acm}

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/krkn-operator-acm-olm.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT

rm -rf "$output_dir"
mkdir -p "$output_dir"

(
  cd "$work_dir"
  "$kustomize" build "$repo_root/config/manifests" |
    "$operator_sdk" generate bundle \
      --kustomize-dir "$repo_root/config/manifests" \
      --output-dir "$output_dir" \
      --package krkn-operator-acm \
      --version "$version" \
      --channels "$channel" \
      --default-channel "$channel" \
      --manifests \
      --metadata \
      --overwrite
)

csv_file="$output_dir/manifests/krkn-operator-acm.clusterserviceversion.yaml"
export OPERATOR_IMAGE="$operator_image"
export MIN_KUBE_VERSION="$min_kube_version"

yq -i \
  '.metadata.annotations.containerImage = strenv(OPERATOR_IMAGE) |
   .spec.minKubeVersion = strenv(MIN_KUBE_VERSION) |
   (.spec.install.spec.deployments[] | select(.name == "krkn-operator-acm-controller-manager") | .spec.template.spec.containers[] | select(.name == "manager") | .image) = strenv(OPERATOR_IMAGE) |
   .spec.relatedImages = [{"name": "krkn-operator-acm", "image": strenv(OPERATOR_IMAGE)}]' \
  "$csv_file"

cp "$repo_root/config/manifests/dependencies.yaml" "$output_dir/metadata/dependencies.yaml"
"$operator_sdk" bundle validate "$output_dir"
