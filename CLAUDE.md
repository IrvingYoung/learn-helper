# CLAUDE.md

## 项目概述

LLM Wiki - AI 维护的个人知识库。用户通过对话让 AI 管理知识树，所有写入操作需确认。

## 技术栈

- Frontend: React 19 + Vite + Tailwind + SWR
- Backend: Go + Chi + SQLite (modernc.org/sqlite)
- AI: Claude/DeepSeek via provider abstraction

## 项目结构

```
learn-helper/
├── frontend/       # React + Vite
├── backend/         # Go + Chi + SQLite
├── docs/            # Design docs
```

## 开发

```bash
cd backend && go run ./cmd/server  # starts on :8080
cd frontend && npm run dev  # starts on :3000
```

## 关键约定

- All AI writes require user confirmation
- Wiki pages stored in `wiki_pages` table
- AI role: `wiki_maintainer`
- SSE streaming for AI chat

## AI Role

Single role: wiki_maintainer. Manages knowledge tree, reads/writes wiki_pages, and auto-maintains overview page.