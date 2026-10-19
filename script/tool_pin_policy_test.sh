#!/usr/bin/env bash
set -euo pipefail

readonly tool_module="golang.org/x/vuln"
readonly tool_package="${tool_module}/cmd/govulncheck"
readonly tool_version="v1.6.0"
readonly module_sum="${tool_module} ${tool_version} h1:FeMO9Rm/HwyduOztbvKcOw+zvDEPr4I4aQNSfevFcKY="
readonly module_file_sum="${tool_module} ${tool_version}/go.mod h1:bWlG2493/sjR7ksvicBgMrznH3eYQEyK8ifUYBrqUbg="
readonly tools_dir="tools"
readonly tools_go_mod="${tools_dir}/go.mod"
readonly tools_go_sum="${tools_dir}/go.sum"
readonly workflow=".github/workflows/conformance.yml"

fail() {
  printf 'tool pin policy: FAIL: %s\n' "$*" >&2
  exit 1
}

[[ "$(grep -Fxc "tool ${tool_package}" "$tools_go_mod")" == 1 ]] ||
  fail "${tools_go_mod} must declare exactly one ${tool_package} tool"
[[ "$(grep -Ec '^[[:space:]]*golang\.org/x/vuln[[:space:]]+' "$tools_go_mod")" == 1 ]] ||
  fail "${tools_go_mod} must contain exactly one ${tool_module} requirement"
[[ "$(grep -Ec '^[[:space:]]*golang\.org/x/vuln v1\.6\.0 // indirect$' "$tools_go_mod")" == 1 ]] ||
  fail "${tools_go_mod} must require ${tool_module} ${tool_version}"
[[ "$(grep -Ec '^golang\.org/x/vuln[[:space:]]+' "$tools_go_sum")" == 2 ]] ||
  fail "${tools_go_sum} must contain exactly two ${tool_module} checksum entries"
[[ "$(grep -Fxc "$module_sum" "$tools_go_sum")" == 1 ]] ||
  fail "${tools_go_sum} must contain the reviewed ${tool_module} module checksum"
[[ "$(grep -Fxc "$module_file_sum" "$tools_go_sum")" == 1 ]] ||
  fail "${tools_go_sum} must contain the reviewed ${tool_module} go.mod checksum"
[[ "$(grep -Ec '^[[:space:]]*golang\.org/x/vuln[[:space:]]+' go.mod)" == 0 ]] ||
  fail "root go.mod must not depend on ${tool_module}"
[[ "$(grep -Ec '^golang\.org/x/vuln[[:space:]]+' go.sum)" == 0 ]] ||
  fail "root go.sum must not contain ${tool_module} checksums"
[[ "$(grep -Ec '^[[:space:]]*- run: go -C tools tool govulncheck -C \.\. \./\.\.\.$' "$workflow")" == 1 ]] ||
  fail "conformance must invoke exactly one module-declared govulncheck"
if grep -Eq 'govulncheck@|go (install|run)[[:space:]]+golang\.org/x/vuln/cmd/govulncheck' "$workflow"; then
  fail "conformance contains a mutable or bypassing govulncheck selector"
fi

resolved_version="$(go -C "$tools_dir" list -mod=readonly -m -f '{{.Version}}' "$tool_module")"
readonly resolved_version
[[ "$resolved_version" == "$tool_version" ]] ||
  fail "module graph resolved ${tool_module}@${resolved_version}, want ${tool_version}"

test_root="$(mktemp -d "${TMPDIR:-/tmp}/beamfall-tool-pin.XXXXXX")"
readonly test_root
cleanup() {
  chmod -R u+w "$test_root" 2>/dev/null || true
  rm -rf "$test_root"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
readonly module_cache="$test_root/mod-cache"
readonly cold_cache="$test_root/cold-cache"
readonly offline_cache="$test_root/offline-cache"
mkdir -p "$module_cache" "$cold_cache" "$offline_cache"

cold_binary="$(
  GOMODCACHE="$module_cache" GOCACHE="$cold_cache" GOFLAGS=-mod=readonly \
    go -C "$tools_dir" tool -n govulncheck
)"
readonly cold_binary
[[ -x "$cold_binary" ]] || fail "cold-cache tool build did not produce an executable"
cp "$cold_binary" "$test_root/govulncheck-cold"
cold_version="$("$cold_binary" -version)"
readonly cold_version

offline_binary="$(
  GOMODCACHE="$module_cache" GOCACHE="$offline_cache" GOFLAGS=-mod=readonly \
    GOPROXY=off GOSUMDB=off go -C "$tools_dir" tool -n govulncheck
)"
readonly offline_binary
[[ -x "$offline_binary" ]] || fail "offline-after-prime tool build did not produce an executable"
offline_version="$("$offline_binary" -version)"
readonly offline_version

cmp -s "$test_root/govulncheck-cold" "$offline_binary" ||
  fail "cold-cache and offline-after-prime binaries differ"
[[ "$cold_version" == "$offline_version" ]] ||
  fail "cold-cache and offline-after-prime versions differ"
grep -Fqx "Scanner: govulncheck@${tool_version}" <<<"$offline_version" ||
  fail "offline executable did not report govulncheck@${tool_version}"

echo "tool pin policy: PASS"
