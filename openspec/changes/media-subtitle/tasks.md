## 1. Build

- [x] 1.1 Add `Bundle.MediaSubtitle` and its validation
- [x] 1.2 Add `RenderMediaSubtitle` and `MediaSubtitleLocales`
- [x] 1.3 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests: validation accepts declared and structural placeholders, rejects unknown
  fields, placeholder-free segments and missing English; rendering drops incomplete segments
- [x] 2.2 `openspec validate --all --strict` passes
- [x] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
