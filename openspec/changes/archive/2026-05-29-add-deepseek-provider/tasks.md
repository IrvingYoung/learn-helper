## 1. Create DeepSeek Provider Implementation

- [x] 1.1 Create `backend/internal/ai/deepseek.go` file
- [x] 1.2 Implement `DeepSeekProvider` struct with apiKey and model fields
- [x] 1.3 Implement `Chat` method with DeepSeek API endpoint
- [x] 1.4 Implement `StreamChat` method with SSE handling
- [x] 1.5 Add DeepSeek response types (DeepSeekRequest, DeepSeekResponse, DeepSeekStreamResponse)

## 2. Create Provider Factory

- [x] 2.1 Add `NewProvider` factory function to `backend/internal/ai/provider.go`
- [x] 2.2 Support "claude" and "deepseek" provider types
- [x] 2.3 Return error for unknown provider types

## 3. Configuration Support

- [x] 3.1 Add `Provider` field to handler configuration
- [x] 3.2 Update handler to use factory for provider creation
- [x] 3.3 Add DeepSeek model default ("deepseek-chat")

## 4. Testing

- [x] 4.1 Write unit tests for DeepSeekProvider.Chat
- [x] 4.2 Write unit tests for DeepSeekProvider.StreamChat
- [x] 4.3 Write tests for provider factory