import { useState, useCallback, useEffect, useMemo } from 'react';
import { Button, Drawer } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { TodoTable } from '../components/TodoTable';
import { TodoFilter } from '../components/TodoFilter';
import { TodoDetailPanel } from '../components/TodoDetailPanel';
import { TodoForm } from '../components/TodoForm';
import { useTodos } from '../hooks/useTodos';
import { getTodo, updateTodo } from '../api/todos';
import type { TodoFilters, Category, TodoSummary } from '../types';
import './TodoListPage.css';

function useIsMobile() {
  const [isMobile, setIsMobile] = useState(() => window.innerWidth <= 768);
  useEffect(() => {
    const handler = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  }, []);
  return isMobile;
}

export function TodoListPage() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [filters, setFilters] = useState<TodoFilters>({ sort_by: 'created_at', sort_order: 'desc', status: 'open,in_progress' });
  const [selectedTodoId, setSelectedTodoId] = useState<number | null>(null);
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [prerequisiteForId, setPrerequisiteForId] = useState<number | undefined>(undefined);
  const { data, isLoading } = useTodos(filters);

  const lockedPrerequisite = useMemo<TodoSummary | undefined>(() => {
    if (!prerequisiteForId || !data) return undefined;
    const t = data.items.find((item) => item.id === prerequisiteForId);
    return t ? { id: t.id, code: t.code, title: t.title } : undefined;
  }, [prerequisiteForId, data]);

  const handleFilterChange = useCallback((newFilters: Partial<TodoFilters>) => {
    setFilters((prev) => ({ ...prev, ...newFilters }));
  }, []);

  const handleSortChange = useCallback((field: string | undefined, order: 'asc' | 'desc' | undefined) => {
    setFilters((prev) => ({
      ...prev,
      sort_by: field || undefined,
      sort_order: order || undefined,
    }));
  }, []);

  const handlePageChange = useCallback((page: number, pageSize: number) => {
    setFilters((prev) => ({ ...prev, page, page_size: pageSize }));
  }, []);

  const handleSelect = useCallback((id: number) => {
    setSelectedTodoId(id);
    if (isMobile) {
      setMobileDetailOpen(true);
    }
  }, [isMobile]);

  const handleNavigate = useCallback((id: number) => {
    if (id === 0) {
      setSelectedTodoId(null);
    } else {
      setSelectedTodoId(id);
    }
  }, []);

  const handleCreate = () => {
    setEditingId(null);
    setPrerequisiteForId(undefined);
    setFormOpen(true);
  };

  const handleEdit = (id: number) => {
    setEditingId(id);
    setPrerequisiteForId(undefined);
    setFormOpen(true);
  };

  const handleAddPrerequisite = (id: number) => {
    setEditingId(null);
    setPrerequisiteForId(id);
    setFormOpen(true);
  };

  const handlePrerequisiteCreated = useCallback(async (newTodoId: number) => {
    if (prerequisiteForId) {
      const currentTodo = await getTodo(prerequisiteForId);
      const existingIds = currentTodo.depends_on.map((d) => d.id);
      await updateTodo(prerequisiteForId, { depends_on_ids: [...existingIds, newTodoId] });
      setPrerequisiteForId(undefined);
    }
  }, [prerequisiteForId]);

  return (
    <div className="todo-list-page">
      <div className="todo-list-page__header">
        <h2>{t('todo.title')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('todo.newTodo')}
        </Button>
      </div>

      <TodoFilter filters={filters} onChange={handleFilterChange} />

      <div className="todo-list-page__body">
        <div className="todo-list-page__table">
          <TodoTable
            data={data}
            loading={isLoading}
            selectedRowId={selectedTodoId}
            onSelect={handleSelect}
            onEdit={handleEdit}
            onPageChange={handlePageChange}
            onSortChange={handleSortChange}
            page={filters.page || 1}
            pageSize={filters.page_size || 20}
            sortBy={filters.sort_by}
            sortOrder={filters.sort_order}
          />
        </div>

        {!isMobile && (
          <div className="todo-list-page__detail">
            <TodoDetailPanel
              todoId={selectedTodoId}
              onEdit={handleEdit}
              onNavigate={handleNavigate}
              onAddPrerequisite={handleAddPrerequisite}
            />
          </div>
        )}
      </div>

      {isMobile && (
        <Drawer
          title={t('todo.detail')}
          placement="right"
          width="85%"
          open={mobileDetailOpen}
          onClose={() => setMobileDetailOpen(false)}
          className="todo-list-mobile-drawer"
          destroyOnClose={false}
        >
          <TodoDetailPanel
            todoId={selectedTodoId}
            onEdit={handleEdit}
            onNavigate={handleNavigate}
            onAddPrerequisite={handleAddPrerequisite}
          />
        </Drawer>
      )}

      <TodoForm
        open={formOpen}
        todoId={editingId}
        onClose={() => {
          setFormOpen(false);
          setPrerequisiteForId(undefined);
        }}
        defaultCategory={undefined as unknown as Category}
        lockedPrerequisite={lockedPrerequisite}
        onCreated={prerequisiteForId ? handlePrerequisiteCreated : undefined}
      />
    </div>
  );
}
