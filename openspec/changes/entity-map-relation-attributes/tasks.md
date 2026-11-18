## 1. Build

- [x] 1.1 Add `EntityMap.RelationAttributeMap`, `ElementFilter` and `OrdinalOffset`, and let
  `OrdinalField` name a relation attribute key
- [x] 1.2 Document the merge rule and the past-cap `MaxLookups` behavior
- [x] 1.3 Validate the new fields, including on container maps
- [x] 1.4 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Validation vectors for valid and invalid relation attribute maps, filters and offsets
- [x] 2.2 plugin-filmtv and core compile against the change through a local go.work
- [x] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag v0.4.4, re-release the first-party plugins at that ceiling (filmtv also at that floor),
  then repin core
