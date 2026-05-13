import { useState, useCallback, useMemo } from 'react';
import { Button } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { TodoTable } from '../components/TodoTable';
import { TodoFilter } from '../components/TodoFilter';
import { TodoDetailPanel } from '../components/TodoDetailPanel';
import { TodoForm } from '../components/TodoForm';
import { useTodos } from '../hooks/useTodos';
import type { TodoFilters, Category, TodoSummary } from '../types';

export function TodoListPage() {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<TodoFilters>({});
  const [selectedTodoId, setSelectedTodoId] = useState<number | null>(null);
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
  }, []);

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

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <h2 style={{ margin: 0 }}>{t('todo.title')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('todo.newTodo')}
        </Button>
      </div>

      {/* Filter bar */}
      <TodoFilter filters={filters} onChange={handleFilterChange} />

      {/* Main area: left list + right detail */}
      <div style={{ flex: 1, display: 'flex', gap: 16, minHeight: 0, marginTop: 12 }}>
        {/* Left panel: table */}
        <div style={{ flex: 1, overflow: 'auto', minWidth: 0 }}>
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

        {/* Right panel: detail */}
        <div style={{ flex: 1, overflow: 'auto', borderLeft: '1px solid #f0f0f0', background: '#fff', minWidth: 0 }}>
          <TodoDetailPanel
            todoId={selectedTodoId}
            onEdit={handleEdit}
            onNavigate={handleNavigate}
            onAddPrerequisite={handleAddPrerequisite}
          />
        </div>
      </div>

      <TodoForm
        open={formOpen}
        todoId={editingId}
        onClose={() => {
          setFormOpen(false);
          setPrerequisiteForId(undefined);
        }}
        defaultCategory={undefined as unknown as Category}
        lockedPrerequisite={lockedPrerequisite}
      />
    </div>
  );
}
