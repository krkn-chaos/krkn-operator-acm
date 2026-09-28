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
  '  icon:' \
  '    - base64data: dGVzdC1pY29u' \
  '      mediatype: image/png' >"$csv_file"

update_catalog_icon "$catalog_template" "$csv_file" krkn-operator-acm
icon=$(yq -r '.entries[] | select(.schema == "olm.package" and .name == "krkn-operator-acm") | .icon.base64data' "$catalog_template")
mediatype=$(yq -r '.entries[] | select(.schema == "olm.package" and .name == "krkn-operator-acm") | .icon.mediatype' "$catalog_template")
[[ "$icon" == dGVzdC1pY29u && "$mediatype" == image/png ]] || {
  echo "catalog icon was not updated" >&2
  exit 1
}

echo "catalog submission helper tests passed"
