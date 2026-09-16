#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <version> <output-dir>" >&2
  exit 2
}

[[ $# -eq 2 ]] || usage
version=$1
output_dir=$2

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || {
  echo "version must be a semantic version (for example 1.1.0 or 1.1.0-beta.1)" >&2
  exit 2
}

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
min_kube_version=${MIN_KUBE_VERSION:-1.36.0}
channel=${CHANNEL:-stable-acm}
icon_file="$repo_root/config/manifests/bases/krkn-operator-acm-icon.png"
[[ -f "$icon_file" ]] || { echo "bundle icon is required: $icon_file" >&2; exit 1; }

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/krkn-operator-acm-olm.XXXXXX")
icon_base64_file="$work_dir/icon.base64"
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

cat > "$output_dir/bundle.Dockerfile" <<EOF
FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1="registry+v1"
LABEL operators.operatorframework.io.bundle.manifests.v1="manifests/"
LABEL operators.operatorframework.io.bundle.metadata.v1="metadata/"
LABEL operators.operatorframework.io.bundle.package.v1="krkn-operator-acm"
LABEL operators.operatorframework.io.bundle.channels.v1="$channel"
LABEL operators.operatorframework.io.bundle.channel.default.v1="$channel"
LABEL operators.operatorframework.io.metrics.builder="operator-sdk-v1.41.1"
LABEL operators.operatorframework.io.metrics.mediatype.v1="metrics+v1"

COPY manifests/ manifests/
COPY metadata/ metadata/
EOF

csv_file="$output_dir/manifests/krkn-operator-acm.clusterserviceversion.yaml"
export OPERATOR_IMAGE="$operator_image"
export MIN_KUBE_VERSION="$min_kube_version"
base64 < "$icon_file" | tr -d '\n' > "$icon_base64_file"
export ICON_BASE64_FILE="$icon_base64_file"

yq -i \
  '.metadata.annotations.containerImage = strenv(OPERATOR_IMAGE) |
   .spec.minKubeVersion = strenv(MIN_KUBE_VERSION) |
   .spec.icon = [{"base64data": load_str(strenv(ICON_BASE64_FILE)), "mediatype": "image/png"}] |
   (.spec.install.spec.deployments[] | select(.name == "krkn-operator-acm-controller-manager") | .spec.template.spec.containers[] | select(.name == "manager") | .image) = strenv(OPERATOR_IMAGE) |
   .spec.relatedImages = [{"name": "krkn-operator-acm", "image": strenv(OPERATOR_IMAGE)}]' \
  "$csv_file"

yq -e '.spec.minKubeVersion == strenv(MIN_KUBE_VERSION)' "$csv_file" >/dev/null
yq -e '
  .spec.install.spec.deployments[]
  | select(.name == "krkn-operator-acm-controller-manager")
  | .spec.template.spec.containers[]
  | select(.name == "manager")
  | .env[]
  | select(.name == "POD_NAMESPACE" and .valueFrom.fieldRef.fieldPath == "metadata.namespace")
' "$csv_file" >/dev/null
yq -e '
  .spec.install.spec.deployments[]
  | select(.name == "krkn-operator-acm-controller-manager")
  | .spec.template.spec.containers[]
  | select(.name == "manager")
  | .env[]
  | select(.name == "SERVICE_ACCOUNT_NAME" and .valueFrom.fieldRef.fieldPath == "spec.serviceAccountName")
' "$csv_file" >/dev/null

cp "$repo_root/config/manifests/dependencies.yaml" "$output_dir/metadata/dependencies.yaml"
"$operator_sdk" bundle validate "$output_dir"
