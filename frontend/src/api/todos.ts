import { client } from './client';
import type {
  Todo,
  TodoDetail,
  TodoFilters,
  CreateTodoInput,
  UpdateTodoInput,
  PaginatedResponse,
  Comment,
  SetStatusInput,
  TodoGraphResponse,
} from '../types';

export async function listTodos(filters: TodoFilters = {}): Promise<PaginatedResponse<Todo>> {
  const params: Record<string, string> = {};
  if (filters.q) params.q = filters.q;
  if (filters.code) params.code = filters.code;
  if (filters.category) params.category = filters.category;
  if (filters.priority) params.priority = filters.priority;
  if (filters.status) params.status = filters.status;
  if (filters.page) params.page = String(filters.page);
  if (filters.page_size) params.page_size = String(filters.page_size);
  if (filters.sort_by) params.sort_by = filters.sort_by;
  if (filters.sort_order) params.sort_order = filters.sort_order;
  if (filters.tag?.length) params.tag = filters.tag.join(',');

  const res = await client.get('/todos', { params });
  return res.data;
}

export async function getTodo(id: number): Promise<TodoDetail> {
  const res = await client.get(`/todos/${id}`);
  return res.data;
}

export async function getTodoGraph(): Promise<TodoGraphResponse> {
  const res = await client.get('/todos/graph');
  return res.data;
}

export async function createTodo(input: CreateTodoInput): Promise<Todo> {
  const res = await client.post('/todos', input);
  return res.data;
}

export async function updateTodo(id: number, input: UpdateTodoInput): Promise<Todo> {
  const res = await client.patch(`/todos/${id}`, input);
  return res.data;
}

export async function deleteTodo(id: number): Promise<void> {
  await client.delete(`/todos/${id}`);
}

export async function startTodo(id: number): Promise<unknown> {
  const res = await client.post(`/todos/${id}/start`);
  return res.data;
}

export async function completeTodo(id: number, cascadeDependencies = false): Promise<unknown> {
  const res = await client.post(`/todos/${id}/complete`, { cascade_dependencies: cascadeDependencies });
  return res.data;
}

export async function reopenTodo(id: number, cascadeDependents = false): Promise<unknown> {
  const res = await client.post(`/todos/${id}/reopen`, { cascade_dependents: cascadeDependents });
  return res.data;
}

export async function setTodoStatus(id: number, input: SetStatusInput): Promise<unknown> {
  const res = await client.patch(`/todos/${id}/status`, input);
  return res.data;
}

export async function listComments(todoId: number): Promise<Comment[]> {
  const res = await client.get(`/todos/${todoId}/comments`);
  return res.data;
}

export async function createComment(todoId: number, content: string): Promise<Comment> {
  const res = await client.post(`/todos/${todoId}/comments`, { content });
  return res.data;
}

export async function deleteComment(todoId: number, commentId: number): Promise<void> {
  await client.delete(`/todos/${todoId}/comments/${commentId}`);
}

export async function fetchTags(): Promise<string[]> {
  const res = await client.get('/todos/tags');
  return res.data;
}

export async function pinTodo(id: number, pinned: boolean): Promise<Todo> {
  const res = await client.patch(`/todos/${id}/pin`, { pinned });
  return res.data;
}

export async function highlightTodo(id: number, highlighted: boolean): Promise<Todo> {
  const res = await client.patch(`/todos/${id}/highlight`, { highlighted });
  return res.data;
}
