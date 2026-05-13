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
  parent_id: number | null;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface TodoDetail extends Todo {
  depends_on: TodoSummary[];
  depended_by: TodoSummary[];
  duplicate_of: TodoSummary | null;
  duplicates: TodoSummary[];
  parent: TodoSummary | null;
  children: TodoSummary[];
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
  category?: Category;
  priority?: Priority;
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
  parent_id?: number;
  depends_on_ids?: number[];
  duplicate_of_id?: number;
}

export interface UpdateTodoInput {
  title?: string;
  description?: string;
  priority?: Priority;
  tags?: string[];
  due_at?: string | null;
  parent_id?: number | null;
  depends_on_ids?: number[];
  duplicate_of_id?: number | null;
}
