export interface Topic {
  id: number;
  parent_id: number;
  name: string;
  slug: string;
  description: string;
  key_points: string;
  content?: string;
  code_examples?: string;
  common_mistakes?: string;
  difficulty: string;
  sort_order: number;
  exercise_count?: number;
}

export interface SiblingTopic {
  slug: string;
  name: string;
  difficulty: string;
}

export interface BreadcrumbItem {
  slug: string;
  name: string;
}

export interface TopicDetail extends Topic {
  breadcrumb: BreadcrumbItem[];
  prev_topic: SiblingTopic | null;
  next_topic: SiblingTopic | null;
  exercise_count: number;
}

export interface Exercise {
  id: number;
  topic_id: number;
  type: string;
  title: string;
  description: string;
  difficulty: string;
  tags: string;
  hints: string;
  solution_outline: string;
  solution_detail?: string;
  common_errors?: string;
  time_complexity_expected: string;
  space_complexity_expected: string;
  sample_code?: string;
  status?: string;
  mastery_level?: number;
}
