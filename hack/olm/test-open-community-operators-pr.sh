#!/usr/bin/env bash

set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=open-community-operators-pr.sh
source "$script_dir/open-community-operators-pr.sh"

validate_fork "owner/community-operators-prod"
if validate_fork "owner/" >/dev/null 2>&1 || validate_fork "/community-operators-prod" >/dev/null 2>&1; then
  echo "malformed fork coordinates were accepted" >&2
  exit 1
fi

test_dir=$(mktemp -d "${TMPDIR:-/tmp}/test-community-operators.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT
catalog_template="$test_dir/basic.yaml"
csv_file="$test_dir/bundle.clusterserviceversion.yaml"
cp "$script_dir/testdata/catalog-template-basic.yaml" "$catalog_template"
printf '%s\n' \
  'spec:' \
  '  icon:' >"$csv_file"
printf '%s\n' '    - base64data: |' >>"$csv_file"
awk 'BEGIN { for (i = 0; i < 2632; i++) printf "        %s\n", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" }' >>"$csv_file"
printf '%s\n' '      mediatype: image/png' >>"$csv_file"

update_catalog_icon "$catalog_template" "$csv_file" krkn-operator-acm
mediatype=$(yq -r '.entries[] | select(.schema == "olm.package" and .name == "krkn-operator-acm") | .icon.mediatype' "$catalog_template")
[[ "$mediatype" == image/png ]] || {
  echo "catalog icon was not updated" >&2
  exit 1
}
yq -r '.entries[] | select(.schema == "olm.package" and .name == "krkn-operator-acm") | .icon.base64data' "$catalog_template" >"$test_dir/catalog-icon.txt"
yq -r '.spec.icon[0].base64data' "$csv_file" >"$test_dir/csv-icon.txt"
tr -d '\n' <"$test_dir/catalog-icon.txt" >"$test_dir/catalog-icon-normalized.txt"
tr -d '\n' <"$test_dir/csv-icon.txt" >"$test_dir/csv-icon-normalized.txt"
cmp -s "$test_dir/catalog-icon-normalized.txt" "$test_dir/csv-icon-normalized.txt" || {
  echo "catalog icon payload was not preserved" >&2
  exit 1
}

echo "catalog submission helper tests passed"
