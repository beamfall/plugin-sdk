## 1. Build

- [x] 1.1 Add `FeatureParty` to the feature-eligibility vocabulary
- [x] 1.2 Add `PartyTerms` and its validation
- [x] 1.3 Add a CHANGELOG entry and regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests: `party` validates; terms need an English anchor, plurals need singulars, terms need `party`
- [x] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the branch head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
