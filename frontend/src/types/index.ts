export type Category = 'bug' | 'feature' | 'task';
export type Priority = 'p0' | 'p1' | 'p2' | 'p3';
export type RelationType = 'depends_on' | 'duplicate_of';

export interface User {
  id: number;
  display_name: string;
}

export interface AuthMode {
  mode: 'basic' | 'oidc';
}

export type Status = 'open' | 'in_progress' | 'completed';

export interface Todo {
  id: number;
  code: string;
  title: string;
  description: string;
  category: Category;
  priority: Priority;
  status: Status;
  due_at: string | null;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface TodoDetail extends Todo {
  depends_on: TodoSummary[];
  depended_by: TodoSummary[];
  duplicate_of: TodoSummary | null;
  duplicates: TodoSummary[];
}

export interface Comment {
  id: number;
  todo_id: number;
  user_id: number;
  content: string;
  created_at: string;
}

export interface SetStatusInput {
  status: Status;
}

export interface TodoSummary {
  id: number;
  code: string;
  title: string;
  status: Status;
}

export interface TodoGraphNode {
  id: number;
  code: string;
  title: string;
  category: Category;
  priority: Priority;
  status: Status;
  due_at: string | null;
  prerequisite_count: number;
  dependent_count: number;
  component_id: string;
  is_component_root: boolean;
}

export interface TodoGraphEdge {
  source_id: number;
  target_id: number;
}

export interface TodoGraphComponent {
  id: string;
  root_ids: number[];
  root_summaries: TodoSummary[];
  node_ids: number[];
  all_completed: boolean;
}

export interface TodoGraphResponse {
  nodes: TodoGraphNode[];
  edges: TodoGraphEdge[];
  components: TodoGraphComponent[];
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface TodoFilters {
  q?: string;
  code?: string;
  tag?: string[];
  category?: string;
  priority?: string;
  status?: string;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface CreateTodoInput {
  title: string;
  description?: string;
  category: Category;
  priority?: Priority;
  tags?: string[];
  due_at?: string;
  depends_on_ids?: number[];
  duplicate_of_id?: number;
}

export interface UpdateTodoInput {
  title?: string;
  description?: string;
  priority?: Priority;
  tags?: string[];
  due_at?: string | null;
  depends_on_ids?: number[];
  duplicate_of_id?: number | null;
}
