#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <output-dir>" >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
output_dir=$1
[[ -n "$output_dir" && "$output_dir" != "/" && "$output_dir" != "." && "$output_dir" != ".." ]] || usage
mkdir -p "$output_dir"

download_verified() {
  local url=$1
  local expected_sha256=$2
  local destination=$3

  curl --fail --location --retry 3 --output "$destination" "$url"
  printf '%s  %s\n' "$expected_sha256" "$destination" | sha256sum --check --status -
}

operator_sdk_version=v1.41.1
kustomize_version=v5.6.0
yq_version=v4.45.1

download_verified \
  "https://github.com/operator-framework/operator-sdk/releases/download/${operator_sdk_version}/operator-sdk_linux_amd64" \
  348284cbd5298f70e2b0a01f9f86820a3149aa6e7e19272e886a9d5769c7fb69 \
  "$output_dir/operator-sdk"

download_verified \
  "https://github.com/mikefarah/yq/releases/download/${yq_version}/yq_linux_amd64" \
  654d2943ca1d3be2024089eb4f270f4070f491a0610481d128509b2834870049 \
  "$output_dir/yq"

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/krkn-acm-tools.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT
kustomize_archive="$work_dir/kustomize.tar.gz"
download_verified \
  "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${kustomize_version}/kustomize_${kustomize_version}_linux_amd64.tar.gz" \
  54e4031ddc4e7fc59e408da29e7c646e8e57b8088c51b84b3df0864f47b5148f \
  "$kustomize_archive"
tar -xzf "$kustomize_archive" -C "$work_dir"
install -m 0755 "$work_dir/kustomize" "$output_dir/kustomize"

chmod 0755 "$output_dir/operator-sdk" "$output_dir/yq"
