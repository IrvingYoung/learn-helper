## Why

Currently, AI editing a wiki page always uses `update_page`, which requires sending the **entire page content** even for a single sentence change. This wastes output tokens (cost) and risks content drift on large pages, since the LLM must reconstruct the full content without deviation.

## What Changes

- **New `patch_page` action type** that allows AI to incrementally edit wiki page content without sending the full page
- Two operation modes: **replace** (by section heading) and **append** (to end of content)
- Existing `update_page` remains unchanged — AI chooses per scenario
- AI prompt updated to explain when to use `patch_page` vs `update_page`

## Capabilities

### New Capabilities
- `wiki-patch-edit`: Incremental editing of wiki page content via patch operations (replace section, append content)

### Modified Capabilities
*(None — no existing specs need requirement changes)*

## Impact

- **Backend** (`backend/internal/engine/engine.go`): New `execPatchPage` dispatcher (~60 lines)
- **AI provider** (`backend/internal/ai/provider.go`): New action type in tool enum, new params schema, prompt guidance
- **Handler** (`backend/internal/handler/ai.go`): New `PatchPageParams` field in action struct and param resolution
