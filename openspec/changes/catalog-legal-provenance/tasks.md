## 1. Build

- [x] 1.1 Add schema v2 legal metadata, the embedded signed policy and fail-closed verification to `catalogindex`
- [x] 1.2 Add `MigrateV1` and the producer signing helpers
- [x] 1.3 Add the `legal` gate to `sdktest.Admit`
- [x] 1.4 Add `script/gen-docs_test.sh` and run it from `script/gate.sh`
- [x] 1.5 Update the README and the registry admission runbook; regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Mutation, round-trip, signature and keyring tests pass
- [x] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the lane head
- [ ] 2.4 Legal approval and human release approval: NOT_RUN, waived by the owner on 2026-11-10

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
