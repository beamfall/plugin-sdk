## 1. Build

- [x] 1.1 Add `ParsedMedia.VersionLabel`, `ValidVersionLabel`, `ValidateVersionLabel` and `MaxVersionLabelLen`
- [x] 1.2 Check the label contract in `sdktest.RunConformance` and `RunContextConformance`
- [x] 1.3 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests for valid and invalid labels; a conformance run that reds on a broken label
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
