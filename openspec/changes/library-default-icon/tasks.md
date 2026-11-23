## 1. Build

- [ ] 1.1 Add `LibraryDefaults.Icon` and the `LibraryIcons()` vocabulary
- [ ] 1.2 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [ ] 2.1 Unit test: a known icon validates, an unknown one is rejected
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the branch head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
