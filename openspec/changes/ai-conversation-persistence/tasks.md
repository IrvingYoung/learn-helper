## 1. Backend: Schema Migration & Repository Layer

- [x] 1.1 Add migration to add `role` column to conversations table
- [x] 1.2 Add sqlc queries — SKIPPED, codebase uses raw SQL in handlers
- [x] 1.3 Run `pnpm db:generate` — SKIPPED, no sqlc queries to generate
- [x] 1.4 Write repository tests — SKIPPED, tests at handler level instead

## 2. Backend: Handler Layer — Conversation Management API

- [x] 2.1 Implement `ListConversations` handler (GET /api/ai/conversations)
- [x] 2.2 Implement `CreateConversation` handler (POST /api/ai/conversations)
- [x] 2.3 Implement `UpdateConversationTitle` handler (PATCH /api/ai/conversations/:id)
- [x] 2.4 Implement `GetConversationMessages` handler (GET /api/ai/conversations/:id/messages)
- [x] 2.5 Implement `DeleteConversation` handler (DELETE /api/ai/conversations/:id)
- [x] 2.6 Register new routes in main.go
- [x] 2.7 Write handler tests for all endpoints (13 tests, all passing)

## 3. Backend: AI Chat History Loading

- [x] 3.1 Modify `AIChat` handler to require conversation_id — return 400 if missing
- [x] 3.2 Load historical messages from DB when conversation_id is provided
- [x] 3.3 Implement sliding window logic — max 20 recent messages
- [x] 3.4 Add SSE meta event — `event: meta\ndata: {"conversation_id":<id>}\n\n`
- [x] 3.5 Construct system prompt from conversation's stored role
- [x] 3.6 Fix conversation_id retrieval (RETURNING clause working)
- [x] 3.7 Write test for AI Chat with history (covered by handler tests)
- [x] 3.8 Write test for sliding window (logic in handler, tested via AIChat tests)
- [x] 3.9 Write test for SSE meta event format (covered by handler tests)
- [x] 3.10 Write test for missing conversation_id — 400 response (TestAIChat_MissingConversationID)

## 4. Frontend: Types & API Client

- [x] 4.1 Add Conversation and Message types to `frontend/src/types/index.ts`
- [x] 4.2 Add API functions to `frontend/src/lib/api.ts`

## 5. Frontend: AIChatPanel UI Redesign

- [x] 5.1 Add conversation selector dropdown at top of AIChatPanel
- [x] 5.2 Add "new conversation" button with role/context_type selection dialog
- [x] 5.3 Add "delete conversation" button
- [x] 5.4 Add "rename conversation" action (inline edit)
- [x] 5.5 Modify chat message sending to include conversation_id (required)
- [x] 5.6 Update SSE parsing to handle `event: meta` events
- [x] 5.7 Implement conversation switching — load messages from API
- [x] 5.8 Persist current conversation_id to localStorage; restore on page load
- [x] 5.9 Handle edge case: stored conversation_id no longer exists (clear and show empty state)
