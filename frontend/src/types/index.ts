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

export interface ConversationMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  model_provider: string | null;
  token_count: number | null;
  created_at: string;
  plan?: Plan;
}

export type PlanStatus = 'pending' | 'confirmed' | 'executing' | 'completed' | 'rejected' | 'completed_with_failures';
export type ActionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type ActionType = 'create_page' | 'update_page' | 'delete_page' | 'link_pages' | 'move_page';

export interface PlanAction {
  id: string;
  plan_id: string;
  type: ActionType;
  params: Record<string, unknown>;
  depends_on: string[];
  status: ActionStatus;
  result?: string;
  sort_order: number;
  created_at: string;
}

export interface OutlineNode {
  id?: string;
  title: string;
  page_type: 'entity' | 'concept' | 'overview';
  children?: OutlineNode[];
}

export interface Phase {
  title: string;
  description: string;
}

export interface CalibrationQuestion {
  question: string;
  options?: string[];
}

export interface Plan {
  id: string;
  conversation_id: number;
  reasoning: string;
  status: PlanStatus;
  outline?: OutlineNode[];
  phases?: Phase[];
  phase_index?: number;
  total_phases?: number;
  calibration_question?: CalibrationQuestion;
  actions: PlanAction[];
  created_at: string;
  executed_at?: string;
}

export interface ExecutionReport {
  plan_id: string;
  status: PlanStatus;
  outline?: Record<string, unknown>;
  actions: {
    id: string;
    type: ActionType;
    status: ActionStatus;
    result?: Record<string, unknown>;
    error?: string;
  }[];
}