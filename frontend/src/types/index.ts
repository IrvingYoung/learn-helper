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
  links: number[];
  backlinks: number[];
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
  sort_order: number;
  children?: WikiTreeNode[];
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

export interface ToolCallInfo {
  id: string;
  name: string;
  input: Record<string, unknown>;
  output: string;
  error?: string;
  /**
   * Explicit lifecycle state. When absent, the card derives state from
   * output/error (back-compat for older messages without this field).
   * - "pending" : write tool awaiting user permission approval
   * - "running" : tool currently executing
   * - "done"    : tool completed successfully
   * - "error"   : tool failed
   */
  status?: "pending" | "running" | "done" | "error";
}

export interface ConversationMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  model_provider: string | null;
  token_count: number | null;
  created_at: string;
  tool_calls?: ToolCallInfo[];
  skill?: string;
}

export type PlanStatus = 'pending' | 'confirmed' | 'executing' | 'completed' | 'rejected' | 'completed_with_failures';
export type ActionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type ActionType = 'create_page' | 'update_page' | 'delete_page' | 'link_pages' | 'move_page';

export interface ExecutionReport {
  plan_id: string;
  status: PlanStatus;
  outline?: Record<string, { page_id: number; slug: string; path: string }>;
  actions: {
    id: string;
    type: ActionType;
    status: ActionStatus;
    result?: Record<string, unknown>;
    error?: string;
  }[];
}

export interface PermissionRequestItem {
  id: string;
  tool: string;
  input: Record<string, any>;
  preview: string;
}

export interface PermissionRequestEvent {
  request_id: string;
  conversation_id: number;
  items: PermissionRequestItem[];
}

export interface AskUserContext {
  kind: "outline" | "page" | "markdown" | "diff";
  /**
   * Shape of `data` depends on `kind`:
   * - "outline": OutlineNode[] — recursive tree of {id?, title, page_type?, children?}
   * - "markdown": string — raw markdown
   * - "diff":    DiffEntry[] — array of {page_id, before, after, label?}
   * - "page":    { page_id: number; title?: string; content?: string }
   *              The LLM provides `content` directly (taken from its prior
   *              read_page tool call) so the frontend never needs to fetch.
   */
  data: any;
}

export interface AskUserRequestEvent {
  request_id: string;
  conversation_id: number;
  question: string;
  options: string[];
  context?: AskUserContext;
  multi_select: boolean;
  allow_free_text: boolean;
  header?: string;
}

export interface PermissionDecisionInput {
  id: string;
  action: "approve" | "reject" | "edit";
  edited_input?: Record<string, any>;
}