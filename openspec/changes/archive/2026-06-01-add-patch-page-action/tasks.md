## 1. Engine — implement execPatchPage

- [x] 1.1 Add `patch_page` case to `executeAction` switch in `engine.go`
- [x] 1.2 Implement `execPatchPage` with `replace` operation (heading-based section replacement)
- [x] 1.3 Implement `execPatchPage` with `append` operation (content append to end)
- [x] 1.4 Return actionable error when heading not found (list available headings)
- [x] 1.5 Re-parse wiki links after patch (call `updatePageLinks`)

## 2. AI provider — add tool definition

- [x] 2.1 Add `patch_page` to the action type enum in `provider.go`
- [x] 2.2 Add `patch_page_params` JSON schema with `page_id`, `operations` array
- [x] 2.3 Add prompt guidance on when to use `patch_page` vs `update_page`

## 3. AI handler — parse patch_page params

- [x] 3.1 Add `PatchPageParams` field to action struct in `ai.go`
- [x] 3.2 Add `patch_page` case to the param resolution switch

## 4. Verify

- [x] 4.1 Build backend and check compilation
- [x] 4.2 Run any existing tests
