# AI Provider Spec

## ADDED Requirements

### Requirement: DeepSeek Provider Support

The system SHALL support DeepSeek as an AI Provider alongside Claude, implementing the same `AIProvider` interface.

#### Scenario: DeepSeek non-streaming chat
- **WHEN** user sends a chat request with DeepSeek provider configured
- **THEN** system calls DeepSeek API endpoint `https://api.deepseek.com/chat/completions`
- **AND** returns chat response with content and token count

#### Scenario: DeepSeek streaming chat
- **WHEN** user sends a streaming chat request with DeepSeek provider configured
- **THEN** system calls DeepSeek API with stream enabled
- **AND** returns SSE stream chunks containing text deltas
- **AND** signals completion with done=true

#### Scenario: DeepSeek API error handling
- **WHEN** DeepSeek API returns non-200 status
- **THEN** system returns error with API response body as message

### Requirement: Provider Factory

The system SHALL provide a factory function to create the appropriate provider based on configuration.

#### Scenario: Create Claude provider
- **WHEN** configuration specifies `provider: "claude"`
- **THEN** factory returns `*ClaudeProvider` instance

#### Scenario: Create DeepSeek provider
- **WHEN** configuration specifies `provider: "deepseek"`
- **THEN** factory returns `*DeepSeekProvider` instance

#### Scenario: Invalid provider type
- **WHEN** configuration specifies unknown provider type
- **THEN** factory returns error