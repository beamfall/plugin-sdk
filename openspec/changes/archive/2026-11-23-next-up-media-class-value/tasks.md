## 1. Build

- [x] 1.1 Add `OptionalValueSourceKinds()` with `next-up` → `video`, `audio` and validate it
- [x] 1.2 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests: no Value, `video` and `audio` validate; any other Value is rejected
- [x] 2.2 `openspec validate --all --strict` passes
- [x] 2.3 `script/gate.sh` passes on the branch head

## 3. Close

- [x] 3.1 Archive this change after landing, before completing the ticket
