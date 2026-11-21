## 1. Implement

- [x] 1.1 Add and validate named parse hints, embedded tag mappings, and scraper search hint declarations.
- [x] 1.2 Extend parser and scraper trace conformance and cover limits, escaping, and required hints.
- [x] 1.3 Update the Unreleased changelog and regenerate API docs with `script/gen-docs.sh`.

## 2. Verify

- [x] 2.1 Pass `openspec validate --all --strict` and the lane's focused Go checks.
- [ ] 2.2 Run the repository gate at the integration boundary.
- [ ] 2.3 Archive the change after landing.
