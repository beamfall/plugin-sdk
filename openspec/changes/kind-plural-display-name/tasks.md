## 1. Build

- [x] 1.1 Add `EntityKind.PluralDisplayName` with validation
- [ ] 1.2 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [ ] 2.1 Unit tests for a valid plural and each rejection
- [ ] 2.2 `openspec validate --all --strict` and `script/gate.sh` pass

## 3. Release

- [ ] 3.1 Set `Version` to `0.4.7` and record the first-party plugins at `[0.4.7, 0.4.7]`, then tag
- [ ] 3.2 Re-release the first-party plugins at that range and re-record the matrix at their tags
- [ ] 3.3 `openspec archive kind-plural-display-name -y`
