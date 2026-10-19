#!/usr/bin/env python3
"""Reviewer-side validator for collected downstream conformance receipts.

The fanout suite validates a receipt at the moment it writes one. This validates a
receipt somebody hands you afterwards, against
``contracts/downstream-conformance-receipt.schema.json``, with no network and no
downstream credential. Stdlib only, like ``script/godoc2md``.

    contracts/validate-downstream-receipt.py receipts/*.json
    contracts/validate-downstream-receipt.py --controls

``--controls`` is the validator's negative control: it asserts that every
``contracts/testdata/*-accept.json`` fixture is accepted and every
``contracts/testdata/*-reject-*.json`` fixture is rejected, so a validator that had
degraded into a no-op fails instead of passing quietly.
"""

import glob
import json
import os
import re
import sys

CONTRACTS_DIR = os.path.dirname(os.path.abspath(__file__))
SCHEMA_PATH = os.path.join(CONTRACTS_DIR, "downstream-conformance-receipt.schema.json")
TESTDATA_DIR = os.path.join(CONTRACTS_DIR, "testdata")
ABSOLUTE_HTTPS = re.compile(r"^https://[^/\s]+/\S*$")
UTC_DATE_TIME = re.compile(
    r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$"
)


def load_json(path):
    with open(path, encoding="utf-8") as source:
        return json.load(source)


def type_matches(expected, value):
    if expected == "object":
        return isinstance(value, dict)
    if expected == "string":
        return isinstance(value, str)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    raise ValueError("unsupported schema type: %s" % expected)


def check_type(schema, value, where):
    expected = schema.get("type")
    if expected is None or type_matches(expected, value):
        return []
    return ["%s must have type %s" % (where, expected)]


def check_const(schema, value, where):
    if "const" not in schema or value == schema["const"]:
        return []
    return ["%s does not match its schema constant" % where]


def check_enum(schema, value, where):
    if "enum" not in schema or value in schema["enum"]:
        return []
    return ["%s is outside its schema enum" % where]


def check_minimum(schema, value, where):
    if "minimum" not in schema or not isinstance(value, int):
        return []
    if value >= schema["minimum"]:
        return []
    return ["%s is below its schema minimum" % where]


def check_pattern(schema, value, where):
    if "pattern" not in schema or not isinstance(value, str):
        return []
    if re.fullmatch(schema["pattern"], value):
        return []
    return ["%s does not match its schema pattern" % where]


def check_format(schema, value, where):
    fmt = schema.get("format")
    if fmt is None or not isinstance(value, str):
        return []
    if fmt == "uri" and not ABSOLUTE_HTTPS.match(value):
        return ["%s is not an absolute HTTPS URI" % where]
    if fmt == "date-time" and not UTC_DATE_TIME.match(value):
        return ["%s is not an RFC 3339 date-time" % where]
    return []


def check_required(schema, value, where):
    if not isinstance(value, dict):
        return []
    missing = [key for key in schema.get("required", []) if key not in value]
    if not missing:
        return []
    return ["%s is missing: %s" % (where, ", ".join(sorted(missing)))]


def check_no_extras(schema, value, where):
    if not isinstance(value, dict) or schema.get("additionalProperties") is not False:
        return []
    extras = [key for key in value if key not in schema.get("properties", {})]
    if not extras:
        return []
    return ["%s has extra properties: %s" % (where, ", ".join(sorted(extras)))]


SCALAR_CHECKS = (
    check_type,
    check_const,
    check_enum,
    check_minimum,
    check_pattern,
    check_format,
    check_required,
    check_no_extras,
)


def validate(schema, value, where="$"):
    problems = []
    for check in SCALAR_CHECKS:
        problems.extend(check(schema, value, where))
    if problems or not isinstance(value, dict):
        return problems
    properties = schema.get("properties", {})
    for key in sorted(value):
        if key in properties:
            problems.extend(validate(properties[key], value[key], "%s.%s" % (where, key)))
    return problems


def verdict(schema, path):
    try:
        return validate(schema, load_json(path))
    except (OSError, ValueError) as failure:
        return ["%s is not readable receipt JSON: %s" % (path, failure)]


def report(path, problems):
    if not problems:
        print("accept %s" % path)
        return
    print("reject %s" % path)
    for problem in problems:
        print("    %s" % problem)


def validate_paths(schema, paths):
    rejected = 0
    for path in paths:
        problems = verdict(schema, path)
        report(path, problems)
        rejected += 1 if problems else 0
    print("receipts=%d accepted=%d rejected=%d" % (len(paths), len(paths) - rejected, rejected))
    return 1 if rejected else 0


def control_expectation(path):
    return "reject" if "-reject-" in os.path.basename(path) else "accept"


def run_controls(schema):
    paths = sorted(glob.glob(os.path.join(TESTDATA_DIR, "*.json")))
    if not paths:
        print("controls: no fixtures under %s" % TESTDATA_DIR)
        return 1
    unmet = 0
    for path in paths:
        expected = control_expectation(path)
        problems = verdict(schema, path)
        actual = "reject" if problems else "accept"
        mark = "ok" if actual == expected else "UNMET"
        unmet += 0 if actual == expected else 1
        print("%-5s %s expected=%s actual=%s" % (mark, os.path.basename(path), expected, actual))
        for problem in problems:
            print("    %s" % problem)
    print("controls=%d unmet=%d" % (len(paths), unmet))
    return 1 if unmet else 0


def main(argv):
    schema = load_json(SCHEMA_PATH)
    if argv[1:2] == ["--controls"]:
        return run_controls(schema)
    if len(argv) < 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    return validate_paths(schema, argv[1:])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
