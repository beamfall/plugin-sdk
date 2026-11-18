## 1. Build

- [ ] 1.1 Add `EntityMap.ArtworkMap`, `TitlePath` and `ContainerRelations` with validation
- [ ] 1.2 Let a Container map hydrate through `ArtworkMap` or `TitlePath` alone
- [ ] 1.3 `sdktest` container reference traces and the index/predicate path grammar
- [ ] 1.4 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [ ] 2.1 Unit tests for valid and invalid container artwork, title and relation maps
- [ ] 2.2 Core and filmtv compile against the change
- [ ] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag the patch release, re-release the first-party plugins at that ceiling (filmtv also at that floor), then repin core
