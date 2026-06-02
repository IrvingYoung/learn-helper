## ADDED Requirements

### Requirement: OpenCode Go provider implements AIProvider interface
The `OpenCodeProvider` struct SHALL implement the `AIProvider` interface with `Chat` and `StreamChat` methods, using the OpenAI-compatible chat/completions API at `https://api.opencode.ai/v1/chat/completions`.

#### Scenario: Non-streaming chat request
- **WHEN** `Chat` is called with a `ChatRequest` containing messages and tools
- **THEN** the provider sends a POST request to `https://api.opencode.ai/v1/chat/completions` with OpenAI-format JSON body and returns a `ChatResponse` with content and/or tool calls

#### Scenario: Streaming chat request
- **WHEN** `StreamChat` is called with a `ChatRequest`
- **THEN** the provider sends a POST request with `stream: true` and returns a channel of `ChatChunk` values parsed from OpenAI-format SSE events

#### Scenario: Authentication
- **WHEN** any request is made to the API
- **THEN** the request includes `Authorization: Bearer <api_key>` header

### Requirement: OpenCode Go provider is registered in factory
The `NewProvider` factory function SHALL accept `ProviderType "opencode"` and return an `OpenCodeProvider` instance.

#### Scenario: Create provider via factory
- **WHEN** `NewProvider` is called with `ProviderType "opencode"`, an API key, and a model name
- **THEN** it returns an `OpenCodeProvider` configured with the given API key and model

#### Scenario: Invalid provider type
- **WHEN** `NewProvider` is called with an unrecognized provider type
- **THEN** it returns an error listing supported providers (now: opencode, deepseek)

### Requirement: Default model is deepseek-v4-pro
When no model is specified, the OpenCode Go provider SHALL default to `deepseek-v4-pro`.

#### Scenario: Empty model parameter
- **WHEN** an `OpenCodeProvider` is created with an empty model string
- **THEN** the provider uses `deepseek-v4-pro` as the model

### Requirement: Database migration from claude to opencode
On server startup, existing database records with `provider='claude'` SHALL be automatically migrated to `provider='opencode'` with `model_name='deepseek-v4-pro'`.

#### Scenario: Legacy claude config exists
- **WHEN** the server starts and finds an `ai_configs` row with `provider='claude'`
- **THEN** the row is updated to `provider='opencode'` and `model_name='deepseek-v4-pro'`

#### Scenario: No legacy config
- **WHEN** the server starts and no `provider='claude'` rows exist
- **THEN** no migration occurs

### Requirement: Claude provider is removed
The `claude.go` file, `ProviderClaude` constant, `ParseClaudeSSE` function, and all Claude-specific request/response types SHALL be removed from the codebase.

#### Scenario: Provider type constant
- **WHEN** the `ProviderType` constants are defined
- **THEN** only `ProviderOpenCode` and `ProviderDeepSeek` exist (no `ProviderClaude`)

#### Scenario: Claude SSE parser removed
- **WHEN** the `provider.go` file is reviewed
- **THEN** `ParseClaudeSSE` function does not exist

### Requirement: Frontend configuration updated
The AI configuration UI SHALL replace the "Claude" provider option with "OpenCode Go" and update the default model accordingly.

#### Scenario: Provider dropdown options
- **WHEN** the user opens the AI configuration page
- **THEN** the provider dropdown shows "OpenCode Go" and "DeepSeek" (not "Claude")

#### Scenario: Default model on select OpenCode Go
- **WHEN** the user selects "OpenCode Go" as provider
- **THEN** the model field defaults to `deepseek-v4-pro`
