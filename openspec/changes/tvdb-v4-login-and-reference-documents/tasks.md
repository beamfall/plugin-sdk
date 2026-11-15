## 1. Build

- [x] 1.1 Add `LoginSpec.OptionalSecretFields` and `OptionalSecret`
- [x] 1.2 Add `ScraperSpec.ReferenceDocuments` and `ReferenceDocument`
- [x] 1.3 Validate optional login secrets and reference documents, including `{ref.<name>:<path>}` placeholders
- [x] 1.4 Document multi-condition predicates, `||` alternatives and `{year}`
- [x] 1.5 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests for valid and invalid optional login secrets and reference documents
- [x] 2.2 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag v0.3.6, then repin filmtv and core
