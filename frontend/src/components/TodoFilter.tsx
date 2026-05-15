import { Input, Select, Button } from 'antd';
import { ClearOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { TodoFilters } from '../types';
import './TodoFilter.css';

interface Props {
  filters: TodoFilters;
  onChange: (filters: Partial<TodoFilters>) => void;
}

export function TodoFilter({ filters, onChange }: Props) {
  const { t } = useTranslation();

  return (
    <div className="todo-filter-bar">
      <div className={`filter-control ${filters.q ? 'filter-active' : ''}`}>
        <Input.Search
          placeholder={t('filter.searchTitle')}
          allowClear
          style={{ width: 200 }}
          value={filters.q || ''}
          onChange={(e) => onChange({ q: e.target.value || undefined })}
          onSearch={(value) => onChange({ q: value || undefined })}
        />
      </div>

      <div className={`filter-control ${filters.code ? 'filter-active' : ''}`}>
        <Input
          placeholder={t('filter.codeExact')}
          allowClear
          style={{ width: 140 }}
          value={filters.code || ''}
          onChange={(e) => onChange({ code: e.target.value || undefined })}
        />
      </div>

      <div className={`filter-control ${filters.category ? 'filter-active' : ''}`}>
        <Select
          mode="multiple"
          maxTagCount="responsive"
          placeholder={t('todo.category')}
          allowClear
          style={{ width: 180 }}
          value={filters.category ? filters.category.split(',') : []}
          onChange={(values: string[]) => onChange({ category: values.length > 0 ? values.join(',') : undefined })}
          options={[
            { value: 'bug', label: t('todo.bug') },
            { value: 'feature', label: t('todo.feature') },
            { value: 'task', label: t('todo.task') },
          ]}
        />
      </div>

      <div className={`filter-control ${filters.priority ? 'filter-active' : ''}`}>
        <Select
          mode="multiple"
          maxTagCount="responsive"
          placeholder={t('todo.priority')}
          allowClear
          style={{ width: 180 }}
          value={filters.priority ? filters.priority.split(',') : []}
          onChange={(values: string[]) => onChange({ priority: values.length > 0 ? values.join(',') : undefined })}
          options={[
            { value: 'p0', label: 'P0' },
            { value: 'p1', label: 'P1' },
            { value: 'p2', label: 'P2' },
            { value: 'p3', label: 'P3' },
          ]}
        />
      </div>

      <div className={`filter-control ${filters.status ? 'filter-active' : ''}`}>
        <Select
          mode="multiple"
          maxTagCount="responsive"
          placeholder={t('todo.status')}
          allowClear
          style={{ width: 200 }}
          value={filters.status ? filters.status.split(',') : []}
          onChange={(values: string[]) => onChange({ status: values.length > 0 ? values.join(',') : undefined })}
          options={[
            { value: 'open', label: t('todo.open') },
            { value: 'in_progress', label: t('todo.inProgress') },
            { value: 'completed', label: t('todo.completed') },
          ]}
        />
      </div>

      <Button
        className="filter-clear-btn"
        icon={<ClearOutlined />}
        onClick={() => onChange({ q: undefined, code: undefined, category: undefined, priority: undefined, status: undefined, sort_by: undefined, sort_order: undefined })}
      >
        {t('common.clear')}
      </Button>
    </div>
  );
}
