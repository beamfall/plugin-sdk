## 1. Build

- [x] 1.1 Add `sortTitle` to `builtinMediaFields`
- [x] 1.2 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit test: `sortTitle` classifies as a bare builtin route, a mis-cased key is rejected
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
