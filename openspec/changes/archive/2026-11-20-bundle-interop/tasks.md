## 1. Build

- [x] 1.1 Add `Bundle.Interop`, `InteropSpec` and `InteropMapping` with validation
- [x] 1.2 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests for the valid shapes and each rejection
- [x] 2.2 filmtv declares `Interop` and compiles against the change (CORE-2812)
- [x] 2.3 `openspec validate --all --strict` and `script/gate.sh` pass

## 3. Release

- [x] 3.1 Set `Version` to `0.4.6` and record the first-party plugins at `[0.4.6, 0.4.6]`, then tag
- [x] 3.2 Re-release the first-party plugins at that range (CORE-2811, CORE-2812) and re-record the matrix at their tags
- [x] 3.3 `openspec archive bundle-interop -y`
