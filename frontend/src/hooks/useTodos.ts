import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listTodos, getTodo, createTodo, updateTodo, deleteTodo, startTodo, setTodoStatus, completeTodo, reopenTodo, listComments, createComment, deleteComment } from '../api/todos';
import type { TodoFilters, CreateTodoInput, UpdateTodoInput, SetStatusInput } from '../types';

export function useTodos(filters: TodoFilters) {
  return useQuery({
    queryKey: ['todos', filters],
    queryFn: () => listTodos(filters),
  });
}

export function useTodo(id: number) {
  return useQuery({
    queryKey: ['todos', id],
    queryFn: () => getTodo(id),
    enabled: !!id,
  });
}

export function useCreateTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateTodoInput) => createTodo(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useUpdateTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: UpdateTodoInput }) => updateTodo(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useDeleteTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => deleteTodo(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useStartTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => startTodo(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useSetTodoStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: SetStatusInput }) => setTodoStatus(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useCompleteTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, cascade }: { id: number; cascade: boolean }) => completeTodo(id, cascade),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useReopenTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, cascade }: { id: number; cascade: boolean }) => reopenTodo(id, cascade),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['todos'] }),
  });
}

export function useComments(todoId: number) {
  return useQuery({
    queryKey: ['comments', todoId],
    queryFn: () => listComments(todoId),
    enabled: !!todoId,
  });
}

export function useCreateComment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ todoId, content }: { todoId: number; content: string }) => createComment(todoId, content),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ['comments', variables.todoId] });
    },
  });
}

export function useDeleteComment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ todoId, commentId }: { todoId: number; commentId: number }) => deleteComment(todoId, commentId),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ['comments', variables.todoId] });
    },
  });
}
