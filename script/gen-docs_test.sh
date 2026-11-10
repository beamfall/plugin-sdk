#!/usr/bin/env bash
# Fails when the generated reference and LLM docs differ from what
# script/gen-docs.sh produces. It copies the working tree (tracked and
# untracked, non-ignored files) into a scratch directory, regenerates twice and
# compares both runs with the checkout, so a doc comment changed without
# rerunning script/gen-docs.sh, or a non-deterministic generator, fails.
set -euo pipefail
cd "$(dirname "$0")/.."

readonly generated=(
  docs/reference/sdk.md
  docs/reference/catalogindex.md
  docs/reference/compatibility.md
  docs/reference/scrobble.md
  docs/reference/sdktest.md
  llms.txt
  llms-full.txt
)

scratch="$(mktemp -d "${TMPDIR:-/tmp}/beamfall-plugin-sdk-gen-docs.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
trap 'rm -rf "$scratch"; exit 130' INT
trap 'rm -rf "$scratch"; exit 143' TERM

git ls-files -z --cached --others --exclude-standard |
  while IFS= read -r -d '' path; do
    [ -e "$path" ] || continue
    mkdir -p "$scratch/$(dirname "$path")"
    cp -p "$path" "$scratch/$path"
  done

status=0
for run in 1 2; do
  "$scratch/script/gen-docs.sh" >/dev/null
  for path in "${generated[@]}"; do
    if ! cmp -s "$path" "$scratch/$path"; then
      echo "gen-docs drift (run $run): $path differs from script/gen-docs.sh output; rerun script/gen-docs.sh and commit it" >&2
      diff -u "$path" "$scratch/$path" | head -20 >&2 || true
      status=1
    fi
  done
  [ "$status" -eq 0 ] || break
done
[ "$status" -eq 0 ] && echo "gen-docs: no drift"
exit "$status"
