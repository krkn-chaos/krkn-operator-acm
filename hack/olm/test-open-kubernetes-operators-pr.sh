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
  env -u KUBERNETES_OPERATORS_FORK -u GH_TOKEN "${script[@]}" 1.0.0 "$test_dir"

stub_dir="$test_dir/bin"
bundle_dir="$test_dir/bundle"
mkdir -p "$stub_dir" "$bundle_dir/manifests" "$bundle_dir/metadata"
printf '%s\n' 'kind: ClusterServiceVersion' \
  >"$bundle_dir/manifests/operator.clusterserviceversion.yaml"
printf '%s\n' '---' >"$bundle_dir/metadata/annotations.yaml"
export TEST_CALL_LOG="$test_dir/calls.log"

cat >"$stub_dir/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'gh' >>"$TEST_CALL_LOG"
printf ' %q' "$@" >>"$TEST_CALL_LOG"
printf '\n' >>"$TEST_CALL_LOG"

case "${1:-} ${2:-}" in
  "repo clone")
    mkdir -p "$4"
    ;;
  "pr list")
    ;;
  "pr create")
    ;;
  "auth setup-git")
    ;;
  *)
    echo "unexpected gh command: $*" >&2
    exit 1
    ;;
esac
EOF
cat >"$stub_dir/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'git' >>"$TEST_CALL_LOG"
printf ' %q' "$@" >>"$TEST_CALL_LOG"
printf '\n' >>"$TEST_CALL_LOG"

if [[ "${1:-}" == "-C" && "${3:-}" == "remote" && "${4:-}" == "get-url" ]]; then
  exit 1
fi

if [[ "${1:-}" == "-C" && "${3:-}" == "add" ]]; then
  checkout_path=$2
  version_path=$4
  [[ -f "$checkout_path/$version_path/manifests/operator.clusterserviceversion.yaml" ]] || {
    echo "submitted bundle was not copied into the catalog checkout" >&2
    exit 1
  }
  printf 'bundle copied: %s\n' "$version_path" >>"$TEST_CALL_LOG"
fi
EOF
chmod +x "$stub_dir/gh" "$stub_dir/git"

PATH="$stub_dir:$PATH" \
GH_TOKEN=test-token \
KUBERNETES_OPERATORS_FORK=test-owner/community-operators \
KUBERNETES_OPERATORS_REPOSITORY=test-upstream/community-operators \
RUNNER_TEMP="$test_dir" \
  "${script[@]}" 1.0.0 "$bundle_dir"

grep -Fq 'remote add upstream https://github.com/test-upstream/community-operators.git' "$TEST_CALL_LOG"
grep -Fq 'remote set-url origin https://github.com/test-owner/community-operators.git' "$TEST_CALL_LOG"
grep -Fq 'fetch --quiet upstream main' "$TEST_CALL_LOG"
grep -Fq 'checkout --quiet -B automation/krkn-operator-acm-1.0.0 upstream/main' "$TEST_CALL_LOG"
grep -Fq 'bundle copied: operators/krkn-operator-acm/1.0.0' "$TEST_CALL_LOG"
grep -Fq 'commit -m operator:\ update\ krkn-operator-acm\ bundle\ to\ 1.0.0' "$TEST_CALL_LOG"
grep -Fq 'push --force-with-lease origin automation/krkn-operator-acm-1.0.0' "$TEST_CALL_LOG"
grep -Fq 'gh pr create --repo test-upstream/community-operators --head test-owner:automation/krkn-operator-acm-1.0.0' "$TEST_CALL_LOG"

echo "Kubernetes OperatorHub submission validation tests passed"
