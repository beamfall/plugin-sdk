#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
workflow=.github/workflows/conformance.yml
fixtures=script/sdk-consumer-fanout-fixtures

fail() {
  echo "sdk consumer fanout: $*" >&2
  exit 1
}

run_fanout() {
  python3 - <<'PY'
import json
import os
import re
import stat
import sys
import tempfile
import time
import urllib.error
import urllib.request


API_VERSION = "2026-03-10"
CANONICAL_JOB = "conformance"
SHA_PATTERN = re.compile(r"^[0-9a-f]{40}$")
WORKFLOW_PATH = ".github/workflows/conformance.yml"


class FanoutError(Exception):
    pass


def require_env(name):
    value = os.environ.get(name, "")
    if not value:
        raise FanoutError(f"{name} is required")
    return value


def positive_integer(name, default, maximum=None):
    raw = os.environ.get(name, str(default))
    if not raw.isdecimal():
        raise FanoutError(f"{name} must be a positive integer")
    value = int(raw)
    if value < 1:
        raise FanoutError(f"{name} must be a positive integer")
    if maximum is not None and value > maximum:
        raise FanoutError(f"{name} must not exceed {maximum}")
    return value


class GitHubTransport:
    def __init__(self, token):
        self.api_root = os.environ.get("GITHUB_API_URL", "https://api.github.com").rstrip("/")
        self.token = token

    def request(self, method, path, payload=None):
        data = None
        if payload is not None:
            data = json.dumps(payload, separators=(",", ":")).encode()
        request = urllib.request.Request(
            f"{self.api_root}{path}",
            data=data,
            method=method,
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {self.token}",
                "X-GitHub-Api-Version": API_VERSION,
            },
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                status = response.status
                body = response.read()
        except urllib.error.HTTPError as error:
            status = error.code
            body = error.read()
        except urllib.error.URLError as error:
            raise FanoutError(f"{method} {path} transport failed: {error.reason}") from error
        return status, json.loads(body) if body else {}

    @staticmethod
    def now():
        return time.monotonic()

    @staticmethod
    def sleep(seconds):
        time.sleep(seconds)

    @staticmethod
    def finish():
        return


class FixtureTransport:
    def __init__(self, fixture_path, case):
        with open(fixture_path, encoding="utf-8") as source:
            cases = json.load(source)
        if case not in cases:
            raise FanoutError(f"unknown fixture case: {case}")
        self.requests = list(cases[case])
        self.elapsed = 0

    def request(self, method, path, payload=None):
        if not self.requests:
            raise FanoutError(f"unexpected fixture request: {method} {path}")
        expected = self.requests.pop(0)
        if expected["method"] != method or expected["path"] != path:
            raise FanoutError(
                "fixture request mismatch: "
                f"got {method} {path}, "
                f"want {expected['method']} {expected['path']}"
            )
        if "request" in expected and expected["request"] != payload:
            raise FanoutError(f"fixture request body mismatch for {method} {path}")
        trace_path = os.environ.get("SDK_FANOUT_FIXTURE_TRACE")
        if trace_path:
            with open(trace_path, "a", encoding="utf-8") as trace:
                trace.write(f"{method} {path}\n")
        return expected["status"], expected.get("response", {})

    def now(self):
        return self.elapsed

    def sleep(self, seconds):
        self.elapsed += seconds

    def finish(self):
        if self.requests:
            raise FanoutError(f"{len(self.requests)} fixture requests were not consumed")


def api_json(transport, method, path, payload=None):
    status, body = transport.request(method, path, payload)
    if status != 200:
        raise FanoutError(f"{method} {path} returned HTTP {status}, want 200")
    if not isinstance(body, dict):
        raise FanoutError(f"{method} {path} returned a non-object response")
    return body


def validate_sha(value, label):
    if not isinstance(value, str) or not SHA_PATTERN.fullmatch(value):
        raise FanoutError(f"{label} is not a full commit SHA")
    return value


def validate_run_identity(run, run_id, consumer_sha):
    expected = {
        "id": run_id,
        "event": "workflow_dispatch",
        "head_branch": "main",
        "head_sha": consumer_sha,
        "path": WORKFLOW_PATH,
    }
    for field, value in expected.items():
        if run.get(field) != value:
            raise FanoutError(
                f"workflow run {run_id} has stale or mismatched {field}: "
                f"got {run.get(field)!r}, want {value!r}"
            )


def wait_for_run(transport, repository, run_id, consumer_sha, timeout_seconds, poll_seconds):
    run_path = f"/repos/{repository}/actions/runs/{run_id}"
    deadline = transport.now() + timeout_seconds
    while transport.now() < deadline:
        run = api_json(transport, "GET", run_path)
        validate_run_identity(run, run_id, consumer_sha)
        if run.get("status") == "completed":
            return run
        remaining = deadline - transport.now()
        transport.sleep(min(poll_seconds, remaining))
    raise FanoutError(f"workflow run {run_id} timed out after {timeout_seconds // 60} minutes")


def canonical_job(transport, repository, run_id):
    jobs_path = f"/repos/{repository}/actions/runs/{run_id}/jobs?filter=latest&per_page=100"
    response = api_json(transport, "GET", jobs_path)
    jobs = response.get("jobs")
    if not isinstance(jobs, list):
        raise FanoutError(f"workflow run {run_id} jobs response is missing jobs")
    if response.get("total_count") != len(jobs):
        raise FanoutError(f"workflow run {run_id} has an incomplete jobs page")
    matches = [job for job in jobs if job.get("name") == CANONICAL_JOB]
    if len(matches) != 1:
        raise FanoutError(
            f"workflow run {run_id} has {len(matches)} canonical conformance jobs, want 1"
        )
    return matches[0]


def matches_type(value, expected):
    if expected == "object":
        return isinstance(value, dict)
    if expected == "string":
        return isinstance(value, str)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    raise FanoutError(f"unsupported receipt schema type: {expected}")


def validate_schema_value(schema, value, location="$"):
    expected_type = schema.get("type")
    if expected_type is not None and not matches_type(value, expected_type):
        raise FanoutError(f"receipt {location} must have type {expected_type}")
    if "const" in schema and value != schema["const"]:
        raise FanoutError(f"receipt {location} does not match its schema constant")
    if "enum" in schema and value not in schema["enum"]:
        raise FanoutError(f"receipt {location} is outside its schema enum")
    if "minimum" in schema and value < schema["minimum"]:
        raise FanoutError(f"receipt {location} is below its schema minimum")
    if "pattern" in schema and re.fullmatch(schema["pattern"], value) is None:
        raise FanoutError(f"receipt {location} does not match its schema pattern")
    if schema.get("format") == "uri" and not re.match(r"^https://[^/]+/", value):
        raise FanoutError(f"receipt {location} is not an HTTPS URI")
    if schema.get("format") == "date-time":
        date_time = r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?Z$"
        if re.fullmatch(date_time, value) is None:
            raise FanoutError(f"receipt {location} is not a UTC date-time")
    if expected_type != "object":
        return
    properties = schema.get("properties", {})
    required = schema.get("required", [])
    missing = [key for key in required if key not in value]
    if missing:
        raise FanoutError(f"receipt {location} is missing: {', '.join(missing)}")
    if schema.get("additionalProperties") is False:
        extras = sorted(set(value) - set(properties))
        if extras:
            raise FanoutError(f"receipt {location} has extra properties: {', '.join(extras)}")
    for key, child in value.items():
        validate_schema_value(properties[key], child, f"{location}.{key}")


def write_receipt(receipt, schema_path, receipt_path):
    with open(schema_path, encoding="utf-8") as source:
        schema = json.load(source)
    validate_schema_value(schema, receipt)
    directory = os.path.dirname(receipt_path) or "."
    os.makedirs(directory, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=".receipt-", dir=directory, text=True)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as destination:
            json.dump(receipt, destination, indent=2, sort_keys=True)
            destination.write("\n")
        os.replace(temporary, receipt_path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    os.chmod(receipt_path, stat.S_IRUSR | stat.S_IRGRP | stat.S_IROTH)


def fanout():
    consumer = require_env("SDK_FANOUT_CONSUMER")
    repository = require_env("SDK_FANOUT_REPOSITORY")
    if repository != f"Beamfall/plugin-{consumer}":
        raise FanoutError(f"repository {repository!r} does not match consumer {consumer!r}")
    sdk_sha = validate_sha(require_env("SDK_FANOUT_SDK_SHA"), "SDK SHA")
    token = require_env("SDK_FANOUT_TOKEN")
    receipt_path = require_env("SDK_FANOUT_RECEIPT")
    schema_path = os.environ.get(
        "SDK_FANOUT_RECEIPT_SCHEMA",
        "contracts/downstream-conformance-receipt.schema.json",
    )
    timeout_minutes = positive_integer("SDK_FANOUT_TIMEOUT_MINUTES", 30, maximum=60)
    poll_seconds = positive_integer("SDK_FANOUT_POLL_SECONDS", 10, maximum=60)

    fixture_path = os.environ.get("SDK_FANOUT_FIXTURE")
    if fixture_path:
        transport = FixtureTransport(
            fixture_path,
            require_env("SDK_FANOUT_FIXTURE_CASE"),
        )
    else:
        transport = GitHubTransport(token)

    ref_path = f"/repos/{repository}/git/ref/heads/main"
    reference = api_json(transport, "GET", ref_path)
    consumer_sha = validate_sha(reference.get("object", {}).get("sha"), "consumer SHA")

    dispatch_path = f"/repos/{repository}/actions/workflows/conformance.yml/dispatches"
    payload = {"ref": "main", "inputs": {"sdk_ref": sdk_sha}}
    dispatch = api_json(transport, "POST", dispatch_path, payload)
    run_id = dispatch.get("workflow_run_id")
    if not isinstance(run_id, int) or isinstance(run_id, bool) or run_id < 1:
        raise FanoutError("dispatch response is missing workflow_run_id")

    run = wait_for_run(
        transport,
        repository,
        run_id,
        consumer_sha,
        timeout_minutes * 60,
        poll_seconds,
    )
    job = canonical_job(transport, repository, run_id)
    if run.get("conclusion") != "success":
        raise FanoutError(
            f"workflow run {run_id} conclusion is {run.get('conclusion')!r}, want 'success'"
        )
    if job.get("status") != "completed" or job.get("conclusion") != "success":
        raise FanoutError(
            f"canonical conformance job conclusion is {job.get('conclusion')!r}, want 'success'"
        )

    receipt = {
        "schema_version": 1,
        "consumer": consumer,
        "repository": repository,
        "sdk_sha": sdk_sha,
        "consumer_sha": consumer_sha,
        "workflow_path": WORKFLOW_PATH,
        "workflow_event": "workflow_dispatch",
        "workflow_run_id": run_id,
        "workflow_run_attempt": run.get("run_attempt"),
        "workflow_run_url": run.get("html_url"),
        "workflow_run_conclusion": run.get("conclusion"),
        "canonical_job_name": job.get("name"),
        "canonical_job_id": job.get("id"),
        "canonical_job_conclusion": job.get("conclusion"),
        "dispatched_at": run.get("created_at"),
        "completed_at": job.get("completed_at"),
    }
    write_receipt(receipt, schema_path, receipt_path)
    transport.finish()
    print(
        f"consumer={consumer} repository={repository} sdk_sha={sdk_sha} "
        f"consumer_sha={consumer_sha} run_id={run_id} receipt={receipt_path}"
    )


try:
    fanout()
except (FanoutError, KeyError, TypeError, ValueError) as error:
    print(f"sdk consumer fanout: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

if [ "${1:-}" = fanout ]; then
  run_fanout
  exit
fi

job_block() {
  local job="$1"
  local source="${2:-$workflow}"
  awk -v job="$job" '
    $0 == "  " job ":" { found = 1 }
    found && /^  [[:alnum:]_-]+:$/ && $0 != "  " job ":" { exit }
    found { print }
  ' "$source"
}

job_if_condition() {
  awk '
    function append(value) {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      if (value == "") return
      if (condition != "") condition = condition " "
      condition = condition value
    }
    continuation {
      if (/^      /) {
        line = $0
        sub(/^      /, "", line)
        append(line)
        next
      }
      continuation = 0
    }
    /^    if:[[:space:]]*/ {
      count++
      line = $0
      sub(/^    if:[[:space:]]*/, "", line)
      if (line ~ /^[>|][+-]?$/) {
        continuation = 1
        next
      }
      append(line)
    }
    END {
      if (count != 1 || condition == "") exit 1
      print condition
    }
  '
}

credentialed_jobs() {
  python3 - "$1" <<'PY'
import sys

import yaml


APP_TOKEN_ACTION = "actions/create-github-app-token@"
APP_TOKEN_OUTPUT = "steps.app-token.outputs.token"


def expression_bodies(value):
    cursor = 0
    while (start := value.find("${{", cursor)) != -1:
        index = start + 3
        quote = None
        while index < len(value):
            character = value[index]
            if quote:
                if character != quote:
                    index += 1
                    continue
                if quote == "'" and value[index:index + 2] == "''":
                    index += 2
                    continue
                quote = None
                index += 1
                continue
            if character in ("'", '"'):
                quote = character
                index += 1
                continue
            if value[index:index + 2] == "}}":
                yield value[start + 3:index]
                cursor = index + 2
                break
            index += 1
        else:
            return


def expression_tokens(expression):
    tokens = []
    index = 0
    while index < len(expression):
        character = expression[index]
        if character.isspace():
            index += 1
            continue
        if character.isalpha() or character == "_":
            end = index + 1
            while end < len(expression):
                if not (expression[end].isalnum() or expression[end] in "_-"):
                    break
                end += 1
            tokens.append(("identifier", expression[index:end].lower()))
            index = end
            continue
        if character in ("'", '"'):
            quote = character
            end = index + 1
            value = []
            while end < len(expression):
                if expression[end] != quote:
                    value.append(expression[end])
                    end += 1
                    continue
                if quote == "'" and expression[end:end + 2] == "''":
                    value.append("'")
                    end += 2
                    continue
                end += 1
                break
            tokens.append(("string", "".join(value).lower()))
            index = end
            continue
        tokens.append(("symbol", character))
        index += 1
    return tokens


def is_context(tokens, index, name):
    if tokens[index] != ("identifier", name):
        return False
    return index == 0 or tokens[index - 1] != ("symbol", ".")


def references_member(tokens, index, member):
    if tokens[index + 1:index + 3] == [("symbol", "."), ("identifier", member)]:
        return True
    return tokens[index + 1:index + 4] in (
        [("symbol", "["), ("identifier", member), ("symbol", "]")],
        [("symbol", "["), ("string", member), ("symbol", "]")],
    )


def expression_references_credential(expression):
    tokens = expression_tokens(expression)
    for index in range(len(tokens)):
        if is_context(tokens, index, "secrets"):
            return True
        if not is_context(tokens, index, "github"):
            continue
        if references_member(tokens, index, "token"):
            return True
        if tokens[index - 2:index] == [("identifier", "tojson"), ("symbol", "(")]:
            if tokens[index + 1:index + 2] == [("symbol", ")")]:
                return True
    return False


def references_credential(node):
    if isinstance(node, str):
        expressions = expression_bodies(node)
        credential_expression = any(
            expression_references_credential(expression) for expression in expressions
        )
        return credential_expression or APP_TOKEN_OUTPUT in node
    if isinstance(node, list):
        return any(references_credential(item) for item in node)
    if not isinstance(node, dict):
        return False

    for key, value in node.items():
        if key == "uses" and isinstance(value, str) and value.startswith(APP_TOKEN_ACTION):
            return True
        if references_credential(key) or references_credential(value):
            return True
    return False


def inherits_caller_secrets(job):
    return (
        isinstance(job, dict)
        and isinstance(job.get("uses"), str)
        and job.get("secrets") == "inherit"
    )


with open(sys.argv[1], encoding="utf-8") as source:
    workflow = yaml.safe_load(source) or {}

jobs = workflow.get("jobs", {})
workflow_credential = references_credential(workflow.get("env", {}))
for job_name, job in jobs.items():
    if workflow_credential or inherits_caller_secrets(job) or references_credential(job):
        print(job_name)
PY
}

unsafe_pr_token_jobs() {
  python3 - "$1" <<'PY'
import re
import sys

import yaml


def pull_request_triggered(workflow):
    triggers = workflow.get("on", workflow.get(True))
    if isinstance(triggers, str):
        return triggers == "pull_request"
    if isinstance(triggers, list):
        return "pull_request" in triggers
    return isinstance(triggers, dict) and "pull_request" in triggers


def pull_request_reachable(job):
    condition = str(job.get("if", ""))
    event_equalities = re.findall(
        r"github\.event_name\s*==\s*(['\"])([^'\"]+)\1", condition
    )
    if not event_equalities:
        return True
    return any(event == "pull_request" for _, event in event_equalities)


def token_permissions_are_capped(permissions):
    if permissions == "read-all":
        return True
    if not isinstance(permissions, dict):
        return False
    return permissions.get("contents") == "read" and "write" not in permissions.values()


def checkout_persists_credentials(job):
    for step in job.get("steps", []):
        if not isinstance(step, dict):
            continue
        action = step.get("uses", "")
        if not isinstance(action, str) or not action.startswith("actions/checkout@"):
            continue
        options = step.get("with", {})
        if not isinstance(options, dict):
            return True
        if options.get("persist-credentials") not in (False, "false"):
            return True
    return False


with open(sys.argv[1], encoding="utf-8") as source:
    workflow = yaml.safe_load(source) or {}

if not pull_request_triggered(workflow):
    raise SystemExit

workflow_permissions = workflow.get("permissions")
for job_name, job in workflow.get("jobs", {}).items():
    if not isinstance(job, dict) or not pull_request_reachable(job):
        continue
    permissions = job.get("permissions", workflow_permissions)
    if not token_permissions_are_capped(permissions) or checkout_persists_credentials(job):
        print(job_name)
PY
}

assert_clean_git_config() {
  ! grep -Eiq 'authorization:|credential\.helper|x-access-token|oauth2:|bearer ' "$1"
}

fanout_allowed() {
  [ "$1" = push ] && [ "$2" = refs/heads/main ] && return 0
  [ "$1" = workflow_dispatch ] && [ "$2" = refs/heads/main ]
}

assert_receipt_shas() {
  local case_name="$1"
  local receipt="$2"
  python3 - "$fixtures/api-state-machines.json" "$case_name" "$receipt" <<'PY'
import json
import sys


fixture_path, case_name, receipt_path = sys.argv[1:]
with open(fixture_path, encoding="utf-8") as source:
    requests = json.load(source)[case_name]
with open(receipt_path, encoding="utf-8") as source:
    receipt = json.load(source)

sdk_shas = [
    request.get("request", {}).get("inputs", {}).get("sdk_ref")
    for request in requests
    if request["method"] == "POST" and request["path"].endswith("/dispatches")
]
consumer_shas = [
    request.get("response", {}).get("object", {}).get("sha")
    for request in requests
    if request["method"] == "GET" and request["path"].endswith("/git/ref/heads/main")
]
if len(sdk_shas) != 1 or len(consumer_shas) != 1:
    raise SystemExit(f"fixture {case_name} does not supply exactly one SDK and consumer SHA")
expected_sdk_sha = sdk_shas[0]
expected_consumer_sha = consumer_shas[0]
if expected_sdk_sha == expected_consumer_sha:
    raise SystemExit(f"fixture {case_name} SDK and consumer SHAs are not distinct")
for field, expected in (
    ("sdk_sha", expected_sdk_sha),
    ("consumer_sha", expected_consumer_sha),
):
    actual = receipt.get(field)
    if actual != expected:
        raise SystemExit(
            f"fixture {case_name} receipt {field} is {actual!r}, want {expected!r}"
        )
PY
}

assert_fixture_exhausted() {
  local case_name="$1"
  local trace="$2"
  python3 - "$fixtures/api-state-machines.json" "$case_name" "$trace" <<'PY'
import json
import os
import sys


fixture_path, case_name, trace_path = sys.argv[1:]
with open(fixture_path, encoding="utf-8") as source:
    expected = len(json.load(source)[case_name])
consumed = 0
if os.path.exists(trace_path):
    with open(trace_path, encoding="utf-8") as source:
        consumed = sum(1 for line in source if line.strip())
if consumed != expected:
    raise SystemExit(
        f"fixture {case_name} consumed {consumed} of {expected} expected requests"
    )
PY
}

run_fixture_case() {
  local case_name="$1"
  local expected="$2"
  local timeout_minutes="${3:-30}"
  local expected_error="${4:-}"
  local receipt="$fixture_tmp/$case_name.json"
  local output="$fixture_tmp/$case_name.output"
  local trace="$fixture_tmp/$case_name.requests"
  local status=0
  local actual_output
  local expected_output
  local receipt_mode
  rm -f "$receipt" "$trace"
  SDK_FANOUT_CONSUMER=adult \
  SDK_FANOUT_REPOSITORY=Beamfall/plugin-adult \
  SDK_FANOUT_SDK_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  SDK_FANOUT_TOKEN=fixture-token \
  SDK_FANOUT_TIMEOUT_MINUTES="$timeout_minutes" \
  SDK_FANOUT_POLL_SECONDS=30 \
  SDK_FANOUT_RECEIPT="$receipt" \
  SDK_FANOUT_FIXTURE="$fixtures/api-state-machines.json" \
  SDK_FANOUT_FIXTURE_CASE="$case_name" \
  SDK_FANOUT_FIXTURE_TRACE="$trace" \
    "$0" fanout >"$output" 2>&1 || status=$?
  if [ "$expected" = pass ]; then
    [ "$status" -eq 0 ] || fail "fixture $case_name unexpectedly failed: $(cat "$output")"
    [ -f "$receipt" ] || fail "fixture $case_name did not produce a receipt"
    assert_receipt_shas "$case_name" "$receipt" || fail "fixture $case_name receipt SHA oracle failed"
    receipt_mode="$(stat -c '%a' "$receipt" 2>/dev/null || stat -f '%Lp' "$receipt")"
    [ "$receipt_mode" = 444 ] || fail "fixture $case_name receipt mode is $receipt_mode, want 444"
    return
  fi
  [ "$status" -ne 0 ] || fail "fixture $case_name unexpectedly passed"
  [ ! -e "$receipt" ] || fail "fixture $case_name produced a receipt on failure"
  [ -n "$expected_error" ] || fail "fixture $case_name has no expected error oracle"
  actual_output="$(cat "$output")"
  expected_output="sdk consumer fanout: $expected_error"
  [ "$actual_output" = "$expected_output" ] || fail "fixture $case_name failed with the wrong error
got:
$actual_output
want:
$expected_output"
  assert_fixture_exhausted "$case_name" "$trace" || fail "fixture $case_name request exhaustion oracle failed"
}

want='adult=plugin-adult
audiobook=plugin-audiobook
enrichment=plugin-enrichment
familyarchive=plugin-familyarchive
filmtv=plugin-filmtv
music=plugin-music
photos=plugin-photos
podcasts=plugin-podcasts'
got="$(awk '
  $1 == "-" && $2 == "consumer:" { consumer = $3 }
  $1 == "repository:" { print consumer "=" $2 }
' "$workflow")"
[ "$got" = "$want" ] || fail "consumer matrix mismatch
got:
$got
want:
$want"

conformance="$(job_block conformance)"
fanout="$(job_block notify-consumers)"
[ -n "$conformance" ] || fail "conformance job missing"
[ -n "$fanout" ] || fail "trusted fanout job missing"

checkout_count="$(grep -c 'uses: actions/checkout@' "$workflow" || true)"
persist_count="$(grep -c 'persist-credentials: false' "$workflow" || true)"
[ "$checkout_count" -eq "$persist_count" ] || fail "every checkout must disable credential persistence"
grep -Fq "tags: ['v*']" "$workflow" || fail "tagged-release conformance trigger missing"
grep -Fq 'workflow_dispatch:' "$workflow" || fail "protected manual integration trigger missing"
grep -Fq "github.event_name == 'pull_request'" <<<"$conformance" || fail "PR canary is not wired"
grep -Fq 'run: rm -f .git/config' <<<"$conformance" || fail "checkout Git config is not removed"
grep -Fq 'private repository read returned HTTP' <<<"$conformance" || fail "private-read canary missing"
grep -Fq 'private workflow dispatch returned HTTP' <<<"$conformance" || fail "private-dispatch canary missing"
grep -Fq '"ref":"synthetic-pr-canary-invalid-ref"' <<<"$conformance" || fail "PR dispatch canary could trigger a real workflow"
grep -Fq -- '- run: script/sdk-consumer-fanout_test.sh' <<<"$conformance" || fail "structural test is not wired"
grep -Fq 'FUTURE_REPO_PAT: synthetic-parent-canary' <<<"$conformance" ||
  fail "PR environment isolation lacks a synthetic parent credential"
grep -Fq '[ -z "${FUTURE_REPO_PAT:-}" ]' <<<"$conformance" ||
  fail "PR environment isolation does not assert that the synthetic credential is absent"
proof_allowlist="$(awk '
  /env -i \\$/ { capture = 1; next }
  capture && /bash <</ { exit }
  capture {
    line = $0
    sub(/^[[:space:]]+/, "", line)
    sub(/[[:space:]]+\\$/, "", line)
    print line
  }
' <<<"$conformance")"
expected_proof_allowlist='PATH="$PATH"
HOME="$HOME"
TMPDIR="${TMPDIR:-/tmp}"
LANG="${LANG:-C.UTF-8}"
GITHUB_WORKSPACE="$GITHUB_WORKSPACE"'
[ "$proof_allowlist" = "$expected_proof_allowlist" ] ||
  fail "PR credential proof child environment is not the exact allowlist"

if grep -Eq 'secrets\.|create-github-app-token|Authorization: Bearer|PLUGIN_DISPATCH_TOKEN:' <<<"$conformance"; then
  fail "pull-request-reachable conformance job references an integration credential"
fi

grep -Fq 'needs: conformance' <<<"$fanout" || fail "fanout must depend on credential-free conformance"
expected_guard="(github.event_name == 'push' && github.ref == 'refs/heads/main') || (github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main')"
credentialed="$(credentialed_jobs "$workflow")"
[ -n "$credentialed" ] || fail "workflow has no credentialed integration job"
found_fanout=false
while IFS= read -r job; do
  [ "$job" != notify-consumers ] || found_fanout=true
  block="$(job_block "$job")"
  actual_guard="$(job_if_condition <<<"$block")" ||
    fail "credentialed job $job must have exactly one active job-level guard"
  [ "$actual_guard" = "$expected_guard" ] ||
    fail "credentialed job $job guard mismatch: got '$actual_guard'"
done <<<"$credentialed"
[ "$found_fanout" = true ] || fail "trusted fanout job is not detected as credentialed"

unsafe_fixture="$fixtures/unsafe-pull-request-guard.yml"
unsafe_job="$(job_block notify-consumers "$unsafe_fixture")"
unsafe_guard="$(job_if_condition <<<"$unsafe_job")" ||
  fail "unsafe pull-request guard fixture is invalid"
if [ "$unsafe_guard" = "$expected_guard" ]; then
  fail "unsafe pull-request guard fixture was accepted"
fi
unsafe_workflow_env_fixture="$fixtures/unsafe-workflow-env.yml"
unsafe_workflow_env_credentialed="$(credentialed_jobs "$unsafe_workflow_env_fixture")"
expected_unsafe_workflow_env_credentialed='conformance
notify-consumers'
[ "$unsafe_workflow_env_credentialed" = "$expected_unsafe_workflow_env_credentialed" ] ||
  fail "workflow-level credential mapping escaped the job audit"
unsafe_workflow_env_after_jobs_fixture="$fixtures/unsafe-workflow-env-after-jobs.yml"
unsafe_workflow_env_after_jobs_credentialed="$(credentialed_jobs "$unsafe_workflow_env_after_jobs_fixture")"
[ "$unsafe_workflow_env_after_jobs_credentialed" = "$expected_unsafe_workflow_env_credentialed" ] ||
  fail "workflow-level credential mapping after jobs escaped the job audit"
unsafe_quoted_keys_fixture="$fixtures/unsafe-workflow-env-quoted-keys.yml"
unsafe_quoted_keys_credentialed="$(credentialed_jobs "$unsafe_quoted_keys_fixture")"
[ "$unsafe_quoted_keys_credentialed" = "$expected_unsafe_workflow_env_credentialed" ] ||
  fail "quoted workflow keys escaped the job audit"
unsafe_flow_mapping_fixture="$fixtures/unsafe-job-env-flow-mapping.yml"
unsafe_flow_mapping_credentialed="$(credentialed_jobs "$unsafe_flow_mapping_fixture")"
[ "$unsafe_flow_mapping_credentialed" = conformance ] ||
  fail "flow-style job credential mapping escaped the job audit"
unsafe_github_token_fixture="$fixtures/unsafe-github-token.yml"
unsafe_github_token_credentialed="$(credentialed_jobs "$unsafe_github_token_fixture")"
expected_unsafe_github_token_credentialed='dot-token
bracket-token
single-quoted-token
double-quoted-token'
[ "$unsafe_github_token_credentialed" = "$expected_unsafe_github_token_credentialed" ] ||
  fail "github.token credential mapping escaped the job audit"
unsafe_expression_credentials_fixture="$fixtures/unsafe-expression-credentials.yml"
unsafe_expression_credentials_credentialed="$(credentialed_jobs "$unsafe_expression_credentials_fixture")"
expected_unsafe_expression_credentials_credentialed='format-secret
json-secrets
json-github'
[ "$unsafe_expression_credentials_credentialed" = "$expected_unsafe_expression_credentials_credentialed" ] ||
  fail "indirect or serialized credential context escaped the job audit"
unsafe_inherited_secrets_fixture="$fixtures/unsafe-reusable-workflow-inherited-secrets.yml"
unsafe_inherited_secrets_credentialed="$(credentialed_jobs "$unsafe_inherited_secrets_fixture")"
[ "$unsafe_inherited_secrets_credentialed" = inherited-secrets ] ||
  fail "reusable workflow inherited secrets escaped the job audit"
unsafe_pr_token_fixture="$fixtures/unsafe-pr-token.yml"
unsafe_pr_token_jobs="$(unsafe_pr_token_jobs "$unsafe_pr_token_fixture")"
expected_unsafe_pr_token_jobs='uncapped-token
persisted-token'
[ "$unsafe_pr_token_jobs" = "$expected_unsafe_pr_token_jobs" ] ||
  fail "uncapped or persisted-token pull-request job escaped the workflow audit"
for candidate in .github/workflows/*.yml; do
  unsafe_jobs="$(unsafe_pr_token_jobs "$candidate")"
  [ -z "$unsafe_jobs" ] ||
    fail "$candidate has unsafe pull-request token access in: $unsafe_jobs"
done
grep -Fq 'environment: trusted-integration' <<<"$fanout" || fail "protected integration environment missing"
grep -Fq 'actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1' <<<"$fanout" || fail "GitHub App token action is not immutably pinned"
grep -Fq 'repositories: ${{ matrix.repository }}' <<<"$fanout" || fail "App token is not consumer-repository scoped"
grep -Fq 'permission-actions: write' <<<"$fanout" || fail "App token lacks narrowly scoped dispatch permission"
grep -Fq 'permission-contents: read' <<<"$fanout" || fail "App token cannot resolve the consumer SHA"
grep -Fq 'skip-token-revoke: false' <<<"$fanout" || fail "App token revocation is not enabled"
grep -Fq 'SDK_REF: ${{ github.sha }}' <<<"$fanout" || fail "fanout must dispatch the trusted workflow SHA"
grep -Fq 'SDK_FANOUT_TIMEOUT_MINUTES="$FANOUT_TIMEOUT_MINUTES"' <<<"$fanout" ||
  fail "fanout deadline configuration is not passed to the proof runner"
grep -Fq "FANOUT_TIMEOUT_MINUTES: \${{ vars.SDK_FANOUT_TIMEOUT_MINUTES || '30' }}" <<<"$fanout" ||
  fail "fanout deadline does not default to 30 minutes"
grep -Fq 'script/sdk-consumer-fanout_test.sh fanout' <<<"$fanout" ||
  fail "fanout does not wait for the dispatched run"
grep -Eq '^API_VERSION = "2026-03-10"$' "$0" ||
  fail "fanout does not request workflow dispatch run details"
grep -Fq 'actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02' <<<"$fanout" ||
  fail "receipt upload action is not immutably pinned"
grep -Fq 'if-no-files-found: error' <<<"$fanout" || fail "missing receipt does not fail fanout"
grep -Fq 'overwrite: false' <<<"$fanout" || fail "receipt artifact can be overwritten"
if grep -Fq '204' <<<"$fanout"; then
  fail "fanout treats dispatch acceptance as conformance success"
fi

if grep -Eq 'PLUGIN_DISPATCH_TOKEN:|WEB_REPO_PAT:|pull_request_target|continue-on-error:|persist-credentials: true' "$workflow"; then
  fail "workflow contains a forbidden trust-boundary construct"
fi
if grep -Eq 'set -x|--verbose|echo[^#]*(TOKEN|PRIVATE_KEY)' "$workflow"; then
  fail "workflow may expose integration credentials"
fi
for fixture in "$fixtures"/*.event; do
  event_name=
  ref=
  expected=
  while IFS='=' read -r key value; do
    case "$key" in
      event_name) event_name="$value" ;;
      ref) ref="$value" ;;
      fanout) expected="$value" ;;
    esac
  done < "$fixture"
  [ -n "$event_name" ] && [ -n "$ref" ] && [ -n "$expected" ] || fail "invalid event fixture: $fixture"
  if fanout_allowed "$event_name" "$ref"; then actual=allow; else actual=deny; fi
  [ "$actual" = "$expected" ] || fail "event routing mismatch for $fixture: got $actual, want $expected"
done

assert_clean_git_config "$fixtures/clean.gitconfig" || fail "clean Git config fixture was rejected"
if assert_clean_git_config "$fixtures/credentialed.gitconfig"; then
  fail "credential-bearing sibling Git config fixture was accepted"
fi

fixture_tmp="$(mktemp -d)"
trap 'rm -rf "$fixture_tmp"' EXIT INT TERM
run_fixture_case passing pass
run_fixture_case dispatch-204 fail 30 "POST /repos/Beamfall/plugin-adult/actions/workflows/conformance.yml/dispatches returned HTTP 204, want 200"
run_fixture_case missing fail 30 "dispatch response is missing workflow_run_id"
run_fixture_case missing-job fail 30 "workflow run 107 has 0 canonical conformance jobs, want 1"
run_fixture_case timeout fail 1 "workflow run 102 timed out after 1 minutes"
run_fixture_case cancelled fail 30 "workflow run 103 conclusion is 'cancelled', want 'success'"
run_fixture_case stale-sha fail 30 "workflow run 104 has stale or mismatched head_sha: got 'cccccccccccccccccccccccccccccccccccccccc', want 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'"
run_fixture_case skipped fail 30 "canonical conformance job conclusion is 'skipped', want 'success'"
run_fixture_case failed fail 30 "workflow run 106 conclusion is 'failure', want 'success'"
run_fixture_case invalid-timeout fail 61 "SDK_FANOUT_TIMEOUT_MINUTES must not exceed 60"

echo "sdk consumer fanout: ok"
