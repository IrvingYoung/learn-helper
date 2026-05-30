## ADDED Requirements

### Requirement: Create conversation
The system SHALL allow users to create a new AI conversation by specifying role, context_type, and optionally topic_id, exercise_id, and title. The role SHALL be persisted as a column in the conversations table. If title is not provided, the system SHALL generate a default title based on role.

#### Scenario: Create conversation with topic context
- **WHEN** user sends POST /api/ai/conversations with body `{role: "knowledge_explain", context_type: "topic", topic_id: 1}`
- **THEN** system creates a new conversation record with role="knowledge_explain" and generates default title, returns `{id, title, role, context_type, topic_id, exercise_id, created_at, updated_at}`

#### Scenario: Create conversation with custom title
- **WHEN** user sends POST /api/ai/conversations with body `{role: "problem_solving", context_type: "exercise", exercise_id: 5, title: "排序算法讨论"}`
- **THEN** system creates a new conversation with the provided title

#### Scenario: Create conversation without context
- **WHEN** user sends POST /api/ai/conversations with body `{role: "knowledge_explain", context_type: "dashboard"}`
- **THEN** system creates a new conversation with both topic_id and exercise_id as null

### Requirement: Update conversation
The system SHALL allow users to update a conversation's title by sending PATCH /api/ai/conversations/:id.

#### Scenario: Update conversation title
- **WHEN** user sends PATCH /api/ai/conversations/1 with body `{title: "二叉树深度探讨"}`
- **THEN** system updates the conversation's title and returns the updated conversation

#### Scenario: Update non-existent conversation
- **WHEN** user sends PATCH /api/ai/conversations/9999 with body `{title: "test"}`
- **THEN** system returns 404 with error message

### Requirement: List conversations
The system SHALL return all conversations (no pagination, single-user SQLite), each with id, title, role, context_type, message_count, last_message_preview (truncated to 50 chars), and updated_at, sorted by updated_at descending.

#### Scenario: List conversations returns sorted list
- **WHEN** user sends GET /api/ai/conversations
- **THEN** system returns array of conversations sorted by updated_at descending, each containing id, title, role, context_type, message_count, last_message_preview, updated_at. No pagination — all conversations are returned.

#### Scenario: Empty conversation list
- **WHEN** user sends GET /api/ai/conversations and no conversations exist
- **THEN** system returns empty array `[]`

### Requirement: Get conversation messages
The system SHALL return all messages for a given conversation (no pagination, single-user SQLite), sorted by created_at ascending.

#### Scenario: Get messages for existing conversation
- **WHEN** user sends GET /api/ai/conversations/:id/messages
- **THEN** system returns array of `{id, role, content, model_provider, token_count, created_at}` sorted by created_at ascending. No pagination — all messages are returned.

#### Scenario: Get messages for non-existent conversation
- **WHEN** user sends GET /api/ai/conversations/9999/messages
- **THEN** system returns 404 with error message

### Requirement: Delete conversation
The system SHALL delete a conversation and all its messages (cascade delete).

#### Scenario: Delete existing conversation
- **WHEN** user sends DELETE /api/ai/conversations/:id
- **THEN** system deletes the conversation and all associated messages, returns 204

#### Scenario: Delete non-existent conversation
- **WHEN** user sends DELETE /api/ai/conversations/9999
- **THEN** system returns 404 with error message

### Requirement: Conversation selector UI
The frontend AIChatPanel SHALL display a dropdown selector at the top showing all conversations, with a "new conversation" button and a "delete" button for the current conversation.

#### Scenario: Select existing conversation
- **WHEN** user selects a conversation from the dropdown
- **THEN** system loads that conversation's messages from the API and displays them in the chat area

#### Scenario: Create new conversation via UI
- **WHEN** user clicks the "new conversation" button and selects role/context_type
- **THEN** system creates a new conversation via API, switches to it, and clears the chat area

#### Scenario: Delete conversation via UI
- **WHEN** user triggers deletion of the current conversation
- **THEN** system deletes the conversation via API and switches to the next available conversation, or shows empty state

### Requirement: Persist current conversation across page refresh
The frontend SHALL store the current conversation_id in localStorage and restore it on page load.

#### Scenario: Restore conversation after page refresh
- **WHEN** user refreshes the page with an active conversation
- **THEN** system reads conversation_id from localStorage, loads conversation list and message history from API, and displays the previous conversation

#### Scenario: Conversation deleted externally
- **WHEN** stored conversation_id no longer exists in the backend
- **THEN** system clears the stored ID and shows empty state
