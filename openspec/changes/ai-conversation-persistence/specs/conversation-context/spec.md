## ADDED Requirements

### Requirement: Load history for AI context
When processing a POST /api/ai/chat request with a conversation_id, the system SHALL load historical messages from the messages table for that conversation and include them in the ChatRequest sent to the AI Provider. The system prompt SHALL be constructed independently each request (not loaded from history) based on the conversation's stored role and the request's context field.

#### Scenario: Chat with existing conversation loads history
- **WHEN** user sends POST /api/ai/chat with `{conversation_id: 1, message: "它的遍历方式有哪些？"}`
- **THEN** system loads all prior messages for conversation 1 from the database and prepends them to the user's current message in the ChatRequest, so the AI receives the full conversation context. System prompt is constructed from the conversation's role and the request's context field.

#### Scenario: Chat without conversation_id is rejected
- **WHEN** user sends POST /api/ai/chat without conversation_id
- **THEN** system returns 400 with error message "conversation_id is required". In manual management mode, users must create a conversation first before sending messages.

### Requirement: Sliding window context limit
The system SHALL limit the number of historical messages sent to the AI Provider to a maximum of 20 messages, selecting the most recent ones. Messages beyond this limit SHALL be silently discarded. MVP uses message count only; token budget truncation is a future enhancement.

#### Scenario: Long conversation applies sliding window
- **WHEN** a conversation has 30 historical messages and user sends a new message
- **THEN** system includes only the most recent 20 historical messages plus the current user message in the ChatRequest

#### Scenario: Short conversation sends all history
- **WHEN** a conversation has 5 historical messages and user sends a new message
- **THEN** system includes all 5 historical messages plus the current user message

### Requirement: SSE meta event for conversation_id
The system SHALL emit a meta SSE event at the start of the stream using the SSE `event:` field to distinguish it from content chunks, so the frontend can parse it without confusing it with AI response content.

#### Scenario: New conversation returns ID via SSE
- **WHEN** user sends POST /api/ai/chat and a new conversation is created
- **THEN** system emits `event: meta\ndata: {"conversation_id":<id>}\n\n` as the first SSE event before any content chunks

#### Scenario: Existing conversation returns ID via SSE
- **WHEN** user sends POST /api/ai/chat with conversation_id
- **THEN** system emits `event: meta\ndata: {"conversation_id":<id>}\n\n` as the first SSE event confirming the conversation ID

#### Scenario: Frontend distinguishes meta events from content
- **WHEN** frontend receives an SSE event with `event: meta`
- **THEN** frontend parses the JSON data to extract conversation_id and does NOT append it to the chat message content. Content chunks (no event field, or `event: message`) are appended to the assistant message as before.

### Requirement: Frontend sends conversation_id with chat requests
The frontend SHALL include the current conversation_id in all POST /api/ai/chat requests when a conversation is active.

#### Scenario: Send message in active conversation
- **WHEN** user sends a message while a conversation is active
- **THEN** frontend includes `conversation_id` in the POST /api/ai/chat request body

#### Scenario: Frontend captures conversation_id from SSE meta event
- **WHEN** frontend receives an SSE event with `event: meta` containing `{"conversation_id":<id>}`
- **THEN** frontend stores the conversation_id and uses it for subsequent requests
