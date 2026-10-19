#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

lint_cache="${GOLANGCI_LINT_CACHE:-${TMPDIR:-/tmp}/beamfall-plugin-sdk-golangci-cache}"
lint_lock_file="${lint_cache}.gate-lock"
lint_owner_file="${lint_lock_file}.owner"
lint_lock_timeout="${BEAMFALL_LINT_LOCK_TIMEOUT_SECONDS:-60}"
lint_lock_token=""
coverage_floor_file="${BEAMFALL_COVERAGE_FLOOR_FILE:-testing/coverage-floor.json}"

case "$lint_lock_timeout" in
  ""|*[!0-9]*)
    echo "invalid BEAMFALL_LINT_LOCK_TIMEOUT_SECONDS: $lint_lock_timeout" >&2
    exit 2
    ;;
esac

owner_field() {
  local field="$1"
  awk -F= -v field="$field" '$1 == field {sub(/^[^=]*=/, ""); print; exit}' "$lint_owner_file" 2>/dev/null \
    | tr -d '\r\n' \
    | cut -c1-160 \
    || true
}

release_lint_owner() {
  [ -n "$lint_lock_token" ] || return 0
  [ "$(owner_field token)" = "$lint_lock_token" ] || return 0
  rm -f "$lint_owner_file"
  lint_lock_token=""
}

run_lint_under_lock() {
  local now
  now="$(date +%s)"
  lint_lock_token="$$.${now}"
  printf 'pid=%s\nstarted=%s\ncwd=%s\ntoken=%s\n' "$$" "$now" "$PWD" "$lint_lock_token" >"$lint_owner_file"
  trap release_lint_owner EXIT
  trap 'release_lint_owner; exit 130' INT
  trap 'release_lint_owner; exit 143' TERM
  export GOLANGCI_LINT_CACHE="$lint_cache"
  golangci-lint run ./...
}

report_lint_lock_timeout() {
  local now owner_pid owner_started owner_age owner_cwd
  now="$(date +%s)"
  owner_pid="$(owner_field pid)"
  owner_started="$(owner_field started)"
  owner_cwd="$(owner_field cwd)"
  owner_age="unknown"
  case "$owner_started" in
    ""|*[!0-9]*|?????????????*) ;;
    *)
      owner_age=$((now - owner_started))
      [ "$owner_age" -ge 0 ] || owner_age=0
      ;;
  esac
  echo "golangci-lint lock timeout: waited=${lint_lock_timeout}s owner_pid=${owner_pid:-unknown} owner_age=${owner_age}s owner_cwd=${owner_cwd:-unknown} lock=$lint_lock_file" >&2
}

run_lint_with_lock() {
  local rc
  mkdir -p "$lint_cache"
  set +e
  if command -v lockf >/dev/null 2>&1; then
    lockf -k -s -t "$lint_lock_timeout" "$lint_lock_file" "$0" --lint-under-lock
    rc=$?
  elif command -v flock >/dev/null 2>&1; then
    flock -E 75 -w "$lint_lock_timeout" "$lint_lock_file" "$0" --lint-under-lock
    rc=$?
  else
    echo "golangci-lint serialization requires lockf or flock" >&2
    rc=2
  fi
  set -e
  if [ "$rc" -eq 75 ]; then
    report_lint_lock_timeout
    return 1
  fi
  return "$rc"
}

run_package_coverage() {
  local coverage_profile coverage_rows packages package floor actual
  coverage_profile="$(mktemp "${TMPDIR:-/tmp}/beamfall-plugin-sdk-coverage.XXXXXX")"
  trap 'rm -f "$coverage_profile"' EXIT
  trap 'rm -f "$coverage_profile"; exit 130' INT
  trap 'rm -f "$coverage_profile"; exit 143' TERM

  coverage_rows="$(sed -nE 's/^[[:space:]]*"([^"]+)"[[:space:]]*:[[:space:]]*([0-9]+([.][0-9]+)?),?$/\1 \2/p' "$coverage_floor_file")"
  [ -n "$coverage_rows" ] || { echo "no coverage floors found in $coverage_floor_file" >&2; exit 1; }
  packages="$(go list ./...)"

  while read -r package floor; do
    printf '%s\n' "$packages" | grep -Fqx "$package" || {
      echo "coverage floor names unknown package $package" >&2
      exit 1
    }
  done <<<"$coverage_rows"

  while read -r package; do
    floor="$(printf '%s\n' "$coverage_rows" | awk -v package="$package" '$1 == package {print $2}')"
    [ -n "$floor" ] || { echo "missing coverage floor for $package" >&2; exit 1; }
    go test -covermode=atomic -coverprofile="$coverage_profile" "$package"
    actual="$(go tool cover -func="$coverage_profile" | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
    [ -n "$actual" ] || { echo "could not determine coverage for $package" >&2; exit 1; }
    awk -v package="$package" -v actual="$actual" -v floor="$floor" 'BEGIN {
      if (actual + 0 < floor + 0) {
        printf "coverage for %s is %.1f%%, below %.1f%% floor\n", package, actual, floor > "/dev/stderr"
        exit 1
      }
    }'
  done <<<"$packages"

  rm -f "$coverage_profile"
  trap - EXIT INT TERM
}

if [ "${1:-}" = "--lint-under-lock" ]; then
  run_lint_under_lock
  exit $?
fi

if [ "${1:-}" = "--coverage-only" ]; then
  run_package_coverage
  exit 0
fi

if [ "${BEAMFALL_GATE_SELF_TEST:-0}" != 1 ]; then
  echo "== gate concurrency contract =="
  script/gate_test.sh
fi

if [ "${BEAMFALL_GATE_SELF_TEST:-0}" != 1 ]; then
  # The downstream-fanout proof is only worth what the gate re-runs. Left out of the
  # gate, its receipt attests to one manual invocation at one head and cannot protect
  # the fanout contract from regression thereafter. It uses the fake transport suite,
  # so it needs no network and no downstream credentials.
  echo "== sdk consumer fanout contract =="
  script/sdk-consumer-fanout_test.sh
fi

echo "== release identity =="
go test ./... -run 'TestReleaseVersion(Context|Invariant)$' -count=1
while IFS= read -r head_tag; do
  [ -n "$head_tag" ] || continue
  GITHUB_REF_TYPE=tag GITHUB_REF_NAME="$head_tag" \
    go test ./... -run 'TestReleaseVersionContext$' -count=1
done < <(git tag --points-at HEAD --list 'v*')

echo "== gofmt =="
fmt_out="$(gofmt -l .)"
if [ -n "$fmt_out" ]; then
  echo "gofmt: files need formatting:"
  echo "$fmt_out"
  exit 1
fi

echo "== go vet =="
go vet ./...

echo "== go test =="
go test ./...

if [ "${BEAMFALL_GATE_SELF_TEST:-0}" != 1 ]; then
  echo "== per-package coverage =="
  run_package_coverage
fi

echo "== golangci-lint =="
run_lint_with_lock

echo "ALL GATES GREEN"
