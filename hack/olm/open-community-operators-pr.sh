#!/usr/bin/env bash

set -euo pipefail

package_name=krkn-operator-acm
channel_name=stable-acm

validate_fork() {
  local fork=$1
  [[ "$fork" =~ ^[^/]+/[^/]+$ ]] || {
    echo "COMMUNITY_OPERATORS_FORK must have the form <owner>/<repository>" >&2
    return 2
  }
}

update_catalog_icon() {
  local catalog_template=$1
  local csv_file=$2
  local package_name=$3
  local icon_base64 icon_mediatype package_entries

  icon_base64=$(yq -r '.spec.icon[0].base64data // ""' "$csv_file")
  icon_mediatype=$(yq -r '.spec.icon[0].mediatype // ""' "$csv_file")
  [[ -n "$icon_base64" && -n "$icon_mediatype" ]] || {
    echo "bundle CSV icon is missing" >&2
    return 1
  }
  local icon_base64_file
  icon_base64_file=$(mktemp "${TMPDIR:-/tmp}/krkn-operator-acm-icon.XXXXXX")
  printf '%s' "$icon_base64" >"$icon_base64_file"
  export ICON_BASE64_FILE="$icon_base64_file" ICON_MEDIATYPE="$icon_mediatype" OLM_PACKAGE_NAME="$package_name"
  package_entries=$(yq -r \
    '.entries[] | select(.schema == "olm.package" and .name == strenv(OLM_PACKAGE_NAME)) | .name' \
    "$catalog_template" | wc -l | tr -d ' ')
  [[ "$package_entries" == 1 ]] || {
    echo "expected one package entry, found $package_entries" >&2
    rm -f "$icon_base64_file"
    unset ICON_BASE64_FILE ICON_MEDIATYPE OLM_PACKAGE_NAME
    return 1
  }
  yq -i \
    '(.entries[] | select(.schema == "olm.package" and .name == strenv(OLM_PACKAGE_NAME)) | .icon) = {"base64data": load_str(strenv(ICON_BASE64_FILE)), "mediatype": strenv(ICON_MEDIATYPE)}' \
    "$catalog_template"
  rm -f "$icon_base64_file"
  unset ICON_BASE64_FILE ICON_MEDIATYPE OLM_PACKAGE_NAME
}

main() {
[[ $# -eq 2 ]] || {
  echo "usage: $0 <version> <rendered-bundle>" >&2
  exit 2
}
version=$1
bundle_dir=$2
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]] || {
  echo "invalid release version: $version" >&2
  exit 2
}
[[ -d "$bundle_dir/manifests" && -f "$bundle_dir/metadata/annotations.yaml" ]] || {
  echo "rendered ACM bundle is incomplete: $bundle_dir" >&2
  exit 1
}
: "${COMMUNITY_OPERATORS_FORK:?COMMUNITY_OPERATORS_FORK must be configured}"
: "${GH_TOKEN:?GH_TOKEN must be configured with permission to push to the fork and open upstream PRs}"
validate_fork "$COMMUNITY_OPERATORS_FORK"

git config --global user.name "github-actions[bot]"
git config --global user.email "41898282+github-actions[bot]@users.noreply.github.com"
gh auth setup-git

catalog_repository=${COMMUNITY_OPERATORS_REPOSITORY:-redhat-openshift-ecosystem/community-operators-prod}
fork_owner=${COMMUNITY_OPERATORS_FORK%%/*}

work_dir=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/community-operators.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT
gh repo clone "$COMMUNITY_OPERATORS_FORK" "$work_dir/catalog" >/dev/null
if git -C "$work_dir/catalog" remote get-url upstream >/dev/null 2>&1; then
  git -C "$work_dir/catalog" remote set-url upstream "https://github.com/$catalog_repository.git"
else
  git -C "$work_dir/catalog" remote add upstream "https://github.com/$catalog_repository.git"
fi
git -C "$work_dir/catalog" remote set-url origin "https://github.com/$COMMUNITY_OPERATORS_FORK.git"
git -C "$work_dir/catalog" fetch --quiet upstream main
git -C "$work_dir/catalog" checkout --quiet -B "automation/$package_name-$version" upstream/main

package_dir="$work_dir/catalog/operators/$package_name"
version_dir="$package_dir/$version"
catalog_template="$package_dir/catalog-templates/basic.yaml"
[[ ! -e "$version_dir" ]] || { echo "catalog version already exists: $version" >&2; exit 1; }
[[ -f "$catalog_template" ]] || { echo "catalog template not found: $catalog_template" >&2; exit 1; }

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
previous_bundle=$(bash "$script_dir/channel-head.sh" "$catalog_template" "$package_name" "$channel_name")
[[ "$previous_bundle" =~ ^$package_name\.v[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]] || {
  echo "unable to determine the previous $channel_name bundle: $previous_bundle" >&2
  exit 1
}

mkdir -p "$version_dir"
cp -R "$bundle_dir/manifests" "$version_dir/manifests"
cp -R "$bundle_dir/metadata" "$version_dir/metadata"
csv_file=$(find "$bundle_dir/manifests" -maxdepth 1 -type f -name '*.clusterserviceversion.yaml' -print -quit)
[[ -n "$csv_file" ]] || { echo "bundle CSV not found" >&2; exit 1; }
update_catalog_icon "$catalog_template" "$csv_file" "$package_name"

cat > "$version_dir/release-config.yaml" <<EOF
---
catalog_templates:
  - template_name: basic.yaml
    channels:
      - $channel_name
    replaces: $previous_bundle
EOF

git -C "$work_dir/catalog" add "operators/$package_name/$version" "$catalog_template"
git -C "$work_dir/catalog" commit -m "operator: update $package_name bundle to $version"
git -C "$work_dir/catalog" push --force-with-lease origin "automation/$package_name-$version"

existing_pr=$(gh pr list --repo "$catalog_repository" --head "$fork_owner:automation/$package_name-$version" --state open --json number --jq '.[0].number // empty')
if [[ -n "$existing_pr" ]]; then
  echo "Updated existing catalog PR #$existing_pr"
  exit 0
fi

body_file="$work_dir/pr-body.md"
cat > "$body_file" <<EOF
## Summary

This submission adds the $package_name bundle $version.

The bundle and catalog metadata were generated by the Krkn Operator ACM release workflow.
The included release-config.yaml enables FBC autorelease for the $channel_name channel,
replacing $previous_bundle.

## Release

- Source release: https://github.com/krkn-chaos/krkn-operator-acm/releases/tag/v$version
- Operator image: https://quay.io/repository/krkn-chaos/krkn-operator-acm?tag=v$version
EOF
gh pr create --repo "$catalog_repository" --head "$fork_owner:automation/$package_name-$version" \
  --base main --title "operator $package_name ($version)" --body-file "$body_file"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
