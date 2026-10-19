#!/usr/bin/env bash
set -euo pipefail
set -m
cd "$(dirname "$0")/.."

tmp="$(mktemp -d "${TMPDIR:-/tmp}/beamfall-sdk-gate-test.XXXXXX")"
pids=()
cleanup() {
  local pid
  for pid in ${pids[@]+"${pids[@]}"}; do
    kill -TERM -- "-$pid" 2>/dev/null || true
  done
  for pid in ${pids[@]+"${pids[@]}"}; do
    wait "$pid" 2>/dev/null || true
  done
  rm -rf "$tmp"
}
signal_exit() {
  local status="$1"
  trap - EXIT INT TERM
  cleanup
  exit "$status"
}
trap cleanup EXIT
trap 'signal_exit 130' INT
trap 'signal_exit 143' TERM

register_group() {
  pids+=("$1")
}

stop_group() {
  local pid="$1"
  kill -TERM -- "-$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  if kill -0 -- "-$pid" 2>/dev/null; then
    kill -KILL -- "-$pid" 2>/dev/null || true
    sleep 1
  fi
  ! kill -0 -- "-$pid" 2>/dev/null || { echo "process group $pid survived teardown" >&2; exit 1; }
}

mkdir -p "$tmp/bin"
cat >"$tmp/bin/gofmt" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$tmp/bin/go" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$tmp/bin/golangci-lint" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${FAKE_LINT_FAIL:-0}" = 1 ]; then
  exit 42
fi
active="${GATE_TEST_STATE:?}/active"
if ! mkdir "$active" 2>/dev/null; then
  echo overlap >>"${GATE_TEST_STATE}/overlap"
  exit 80
fi
cleanup_active() { rmdir "$active" 2>/dev/null || true; }
trap cleanup_active EXIT
trap 'cleanup_active; exit 130' INT
trap 'cleanup_active; exit 143' TERM
if [ "${FAKE_LINT_BLOCK:-0}" = 1 ]; then
  : >"${GATE_TEST_STATE}/block-ready"
  while :; do sleep 30; done
fi
sleep 1
EOF
cat >"$tmp/bin/stat" <<'EOF'
#!/usr/bin/env bash
: >"${GATE_TEST_STATE:?}/stat-called"
exit 97
EOF
chmod +x "$tmp/bin/gofmt" "$tmp/bin/go" "$tmp/bin/golangci-lint" "$tmp/bin/stat"

run_gate() {
  env PATH="$tmp/bin:$PATH" GATE_TEST_STATE="$tmp" GOLANGCI_LINT_CACHE="$tmp/cache" \
    FAKE_LINT_BLOCK="${FAKE_LINT_BLOCK:-0}" BEAMFALL_GATE_SELF_TEST=1 \
    BEAMFALL_LINT_LOCK_TIMEOUT_SECONDS="${1:-5}" script/gate.sh
}

hold_lock() {
  local lock_file="$tmp/cache.gate-lock"
  local owner_file="${lock_file}.owner"
  local hold_fifo="$tmp/hold.fifo"
  mkfifo "$hold_fifo"
  (
    exec 9>"$lock_file"
    if command -v lockf >/dev/null 2>&1; then
      lockf -s -t 0 9
    elif command -v flock >/dev/null 2>&1; then
      flock -w 0 9
    else
      echo "gate_test requires lockf or flock" >&2
      exit 1
    fi
    printf 'pid=%s\nstarted=%s\ncwd=/live-owner\ntoken=holder\n' "$$" "$(date +%s)" >"$owner_file"
    exec 8<>"$hold_fifo"
    read -r _ <&8
  ) &
  register_group "$!"
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    [ -s "$owner_file" ] && return 0
    sleep 1
  done
  echo "lock holder did not publish owner evidence" >&2
  exit 1
}

if [ "${1:-}" = "--interrupt-fixture" ]; then
  : "${GATE_INTERRUPT_READY:?}"
  : "${GATE_INTERRUPT_CHILD_PID:?}"
  FAKE_LINT_BLOCK=1 run_gate 5 >"$tmp/fixture-gate.log" 2>&1 &
  fixture_child="$!"
  register_group "$fixture_child"
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    [ -e "$tmp/block-ready" ] && break
    sleep 1
  done
  [ -e "$tmp/block-ready" ] || { echo "fixture blocking linter did not start" >&2; exit 1; }
  printf '%s\n' "$fixture_child" >"$GATE_INTERRUPT_CHILD_PID"
  : >"$GATE_INTERRUPT_READY"
  mkfifo "$tmp/fixture.fifo"
  exec 7<>"$tmp/fixture.fifo"
  read -r _ <&7
  exit 99
fi

echo "== kernel lock is stat-independent =="
if grep -Eq 'stat[[:space:]]+-f[[:space:]]+%m|stat[[:space:]]+-c[[:space:]]+%Y' script/gate.sh; then
  echo "gate reintroduced an mtime-based stat lock fallback" >&2
  exit 1
fi
run_gate 2 >"$tmp/stat-independent.log" 2>&1
[ ! -e "$tmp/stat-called" ] || { echo "kernel lock path invoked stat" >&2; exit 1; }

echo "== concurrent gates serialize =="
run_gate 5 >"$tmp/gate-1.log" 2>&1 &
register_group "$!"
run_gate 5 >"$tmp/gate-2.log" 2>&1 &
register_group "$!"
if ! wait "${pids[0]}"; then
  cat "$tmp/gate-1.log" >&2
  exit 1
fi
if ! wait "${pids[1]}"; then
  cat "$tmp/gate-2.log" >&2
  exit 1
fi
pids=()
[ ! -e "$tmp/overlap" ] || { echo "concurrent linters overlapped" >&2; exit 1; }

echo "== interrupted gate leaves no descendants =="
FAKE_LINT_BLOCK=1 run_gate 5 >"$tmp/interrupted.log" 2>&1 &
interrupted_pid="$!"
register_group "$interrupted_pid"
for _ in 1 2 3 4 5 6 7 8 9 10; do
  [ -e "$tmp/block-ready" ] && break
  sleep 1
done
[ -e "$tmp/block-ready" ] || { echo "blocking linter did not start" >&2; exit 1; }
stop_group "$interrupted_pid"
pids=()
rm -f "$tmp/block-ready"
rmdir "$tmp/active" 2>/dev/null || true

echo "== interrupted harness exits and reaps descendants =="
GATE_INTERRUPT_READY="$tmp/harness-ready" GATE_INTERRUPT_CHILD_PID="$tmp/harness-child" \
  script/gate_test.sh --interrupt-fixture >"$tmp/harness.log" 2>&1 &
harness_pid="$!"
register_group "$harness_pid"
for _ in 1 2 3 4 5 6 7 8 9 10; do
  [ -e "$tmp/harness-ready" ] && break
  sleep 1
done
[ -e "$tmp/harness-ready" ] || { echo "interruption fixture did not become ready" >&2; exit 1; }
harness_child="$(cat "$tmp/harness-child")"
kill -TERM "$harness_pid"
set +e
wait "$harness_pid"
harness_status=$?
set -e
[ "$harness_status" -eq 143 ] || { echo "interrupted harness exited $harness_status, expected 143" >&2; exit 1; }
for _ in 1 2 3; do
  ! kill -0 -- "-$harness_child" 2>/dev/null && break
  sleep 1
done
! kill -0 -- "-$harness_child" 2>/dev/null || { echo "interrupted harness left child group $harness_child" >&2; exit 1; }
pids=()

echo "== stale lock recovers =="
: >"$tmp/cache.gate-lock"
printf 'pid=999999\nstarted=1\ncwd=/stale\ntoken=stale\n' >"$tmp/cache.gate-lock.owner"
run_gate 2 >"$tmp/stale.log" 2>&1
[ ! -e "$tmp/cache.gate-lock.owner" ] || { echo "stale owner metadata survived successful gate" >&2; exit 1; }

echo "== empty stale owner recovers =="
: >"$tmp/cache.gate-lock.owner"
run_gate 2 >"$tmp/empty-stale.log" 2>&1
[ ! -e "$tmp/cache.gate-lock.owner" ] || { echo "empty stale owner metadata survived successful gate" >&2; exit 1; }

echo "== live owner timeout reports evidence =="
hold_lock
holder_pid="${pids[0]}"
[ -z "$(pgrep -P "$holder_pid" 2>/dev/null || true)" ] || { echo "lock holder left a descendant" >&2; exit 1; }
long_cwd="/live-owner"
for _ in 1 2 3 4 5 6 7 8 9 10; do long_cwd="${long_cwd}xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"; done
printf 'pid=%s\nstarted=%s\ncwd=%s\ntoken=holder\n' "$$" "$(date +%s)" "$long_cwd" >"$tmp/cache.gate-lock.owner"
if run_gate 1 >"$tmp/timeout.log" 2>&1; then
  echo "gate unexpectedly acquired a live owner's lock" >&2
  exit 1
fi
grep -q 'golangci-lint lock timeout: waited=' "$tmp/timeout.log"
grep -q "owner_pid=$$" "$tmp/timeout.log"
grep -q 'owner_age=' "$tmp/timeout.log"
grep -q 'owner_cwd=/live-owner' "$tmp/timeout.log"
[ "$(wc -c <"$tmp/timeout.log")" -lt 600 ] || { echo "owner diagnostics are not bounded" >&2; exit 1; }
stop_group "${pids[0]}"
pids=()
rm -f "$tmp/cache.gate-lock.owner"

echo "== lint failures remain fatal =="
if FAKE_LINT_FAIL=1 run_gate 2 >"$tmp/lint-fail.log" 2>&1; then
  echo "gate swallowed a lint failure" >&2
  exit 1
fi

echo "gate_test: PASS"
