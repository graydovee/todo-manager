import { Input, Select, Space, Button } from 'antd';
import { ClearOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { TodoFilters } from '../types';

interface Props {
  filters: TodoFilters;
  onChange: (filters: Partial<TodoFilters>) => void;
}

export function TodoFilter({ filters, onChange }: Props) {
  const { t } = useTranslation();

  return (
    <Space wrap style={{ marginBottom: 16 }}>
      <Input.Search
        placeholder={t('filter.searchTitle')}
        allowClear
        style={{ width: 200 }}
        value={filters.q || ''}
        onChange={(e) => onChange({ q: e.target.value || undefined })}
        onSearch={(value) => onChange({ q: value || undefined })}
      />

      <Input
        placeholder={t('filter.codeExact')}
        allowClear
        style={{ width: 140 }}
        value={filters.code || ''}
        onChange={(e) => onChange({ code: e.target.value || undefined })}
      />

      <Select
        placeholder={t('todo.category')}
        allowClear
        style={{ width: 120 }}
        value={filters.category || undefined}
        onChange={(value) => onChange({ category: value })}
        options={[
          { value: 'bug', label: t('todo.bug') },
          { value: 'feature', label: t('todo.feature') },
          { value: 'task', label: t('todo.task') },
        ]}
      />

      <Select
        placeholder={t('todo.priority')}
        allowClear
        style={{ width: 120 }}
        value={filters.priority || undefined}
        onChange={(value) => onChange({ priority: value })}
        options={[
          { value: 'p0', label: 'P0' },
          { value: 'p1', label: 'P1' },
          { value: 'p2', label: 'P2' },
          { value: 'p3', label: 'P3' },
        ]}
      />

      <Select
        placeholder={t('todo.status')}
        allowClear
        style={{ width: 120 }}
        value={filters.status || undefined}
        onChange={(value) => onChange({ status: value || undefined })}
        options={[
          { value: 'open', label: t('todo.open') },
          { value: 'in_progress', label: t('todo.inProgress') },
          { value: 'completed', label: t('todo.completed') },
        ]}
      />

      <Button
        icon={<ClearOutlined />}
        onClick={() => onChange({ q: undefined, code: undefined, category: undefined, priority: undefined, status: undefined, sort_by: undefined, sort_order: undefined })}
      >
        {t('common.clear')}
      </Button>
    </Space>
  );
}
