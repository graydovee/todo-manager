import { useState, useRef, useCallback, useEffect, useMemo } from 'react';
import { Select, Button, Space, message } from 'antd';
import { useTranslation } from 'react-i18next';
import { listTodos } from '../api/todos';
import { useUpdateTodo } from '../hooks/useTodos';
import { formatDisplayCode } from '../utils/displayCode';
import type { Todo, Category } from '../types';

interface DuplicateSelectorProps {
  currentTodoId: number;
  onSelect: (targetId: number) => void;
  onCancel: () => void;
}

interface TodoOption {
  id: number;
  code: string;
  title: string;
  category: Category;
  status: string;
}

export function DuplicateSelector({ currentTodoId, onSelect, onCancel }: DuplicateSelectorProps) {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<number | undefined>(undefined);
  const [results, setResults] = useState<TodoOption[]>([]);
  const [loading, setLoading] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const updateMutation = useUpdateTodo();

  // Search todos by title or code, excluding current todo and those already marked as duplicate
  const handleSearch = useCallback((query: string) => {
    clearTimeout(timerRef.current);
    if (!query || query.length < 1) {
      setResults([]);
      return;
    }
    timerRef.current = setTimeout(async () => {
      setLoading(true);
      try {
        const res = await listTodos({ q: query, page_size: 20 });
        const filtered = res.items
          .filter((item: Todo) => item.id !== currentTodoId && item.status !== 'duplicate')
          .slice(0, 20)
          .map((item: Todo) => ({
            id: item.id,
            code: item.code,
            title: item.title,
            category: item.category,
            status: item.status,
          }));
        setResults(filtered);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    }, 300);
  }, [currentTodoId]);

  useEffect(() => () => clearTimeout(timerRef.current), []);

  const options = useMemo(
    () => results.map((item) => ({
      value: item.id,
      label: `${formatDisplayCode(item.category, item.code)} - ${item.title}`,
    })),
    [results],
  );

  const handleConfirm = async () => {
    if (!selectedId) return;
    try {
      await updateMutation.mutateAsync({
        id: currentTodoId,
        input: { duplicate_of_id: selectedId },
      });
      message.success(t('confirm.duplicateMarked'));
      onSelect(selectedId);
    } catch {
      message.error(t('confirm.duplicateError'));
    }
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Select
        showSearch
        allowClear
        filterOption={false}
        placeholder={t('todo.enterTodoIds')}
        loading={loading}
        value={selectedId}
        onChange={(value) => setSelectedId(value)}
        onSearch={handleSearch}
        options={options}
        style={{ width: '100%' }}
        notFoundContent={null}
      />
      <Space>
        <Button
          type="primary"
          size="small"
          disabled={!selectedId}
          loading={updateMutation.isPending}
          onClick={handleConfirm}
        >
          {t('todo.done')}
        </Button>
        <Button size="small" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
      </Space>
    </Space>
  );
}
