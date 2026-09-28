#!/usr/bin/env bash

set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
fixture="$script_dir/testdata/catalog-template-basic.yaml"
ambiguous_fixture="$script_dir/testdata/catalog-template-ambiguous.yaml"
selector="$script_dir/channel-head.sh"

cases=(
  "$fixture|stable-acm|krkn-operator-acm.v1.1.0|success"
  "$fixture|stable-missing||failure"
  "$ambiguous_fixture|stable-acm||failure"
)

for test_case in "${cases[@]}"; do
  IFS='|' read -r catalog channel expected outcome <<<"$test_case"
  if [[ "$outcome" == success ]]; then
    actual_head=$(bash "$selector" "$catalog" krkn-operator-acm "$channel")
    [[ "$actual_head" == "$expected" ]] || {
      echo "unexpected head for $channel: $actual_head" >&2
      exit 1
    }
  elif bash "$selector" "$catalog" krkn-operator-acm "$channel" >/dev/null 2>&1; then
    echo "$channel was accepted unexpectedly" >&2
    exit 1
  fi
done

echo "catalog channel head tests passed"
