export interface WikiPage {
  id: number;
  title: string;
  slug: string;
  page_type: 'entity' | 'concept' | 'overview';
  content: string;
  tags: string;
  parent_id: number | null;
  content_status: 'empty' | 'draft' | 'published';
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface WikiTreeNode {
  id: number;
  title: string;
  slug: string;
  page_type: string;
  content_status: string;
  parent_id: number | null;
  path: string;
  sort_order: number;
  children?: WikiTreeNode[];
}

export interface PendingAction {
  type: 'create' | 'update' | 'delete';
  preview: string;
  details?: Record<string, unknown>;
}

export type AIRole = 'wiki_maintainer'
export type AIContextType = 'wiki'

export interface Conversation {
  id: number;
  topic_id: number | null;
  exercise_id: number | null;
  context_type: AIContextType | null;
  role: AIRole | null;
  title: string | null;
  message_count: number;
  last_message_preview: string;
  created_at: string;
  updated_at: string;
}

export interface ConversationMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  model_provider: string | null;
  token_count: number | null;
  created_at: string;
  pending_actions?: PendingAction[];
  confirmed?: boolean;
}