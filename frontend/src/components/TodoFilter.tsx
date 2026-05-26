import { Input, Select, Button, Space, Typography, DatePicker } from 'antd';
import { ClearOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { useTags } from '../hooks/useTodos';
import type { TodoFilters } from '../types';
import './TodoFilter.css';

interface Props {
  filters: TodoFilters;
  onChange: (filters: Partial<TodoFilters>) => void;
}

export function TodoFilter({ filters, onChange }: Props) {
  const { t } = useTranslation();
  const { data: tags, isError: tagsError, refetch: refetchTags } = useTags();

  return (
    <div className="todo-filter-bar">
      {/* Row 1: Search controls (title search + code exact) */}
      <div className="filter-bar__row">
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

        <Button
          className="filter-clear-btn"
          size="small"
          icon={<ClearOutlined />}
          onClick={() => onChange({ q: undefined, code: undefined })}
        >
          {t('common.clear')}
        </Button>
      </div>

      {/* Row 2: Filter controls (tag, category, priority, status, updated_at) */}
      <div className="filter-bar__row">
        <div className={`filter-control ${filters.tag && filters.tag.length > 0 ? 'filter-active' : ''}`}>
          {tagsError ? (
            <Space>
              <Typography.Text type="danger">{t('filter.tagLoadError')}</Typography.Text>
              <Button size="small" icon={<ReloadOutlined />} onClick={() => refetchTags()}>
                {t('filter.tagRetry')}
              </Button>
            </Space>
          ) : (
            <Select
              mode="multiple"
              maxTagCount="responsive"
              placeholder={t('filter.tagPlaceholder')}
              allowClear
              style={{ width: 200 }}
              value={filters.tag || []}
              onChange={(values: string[]) => onChange({ tag: values.length > 0 ? values : undefined })}
              options={(tags || []).map((tag) => ({ value: tag, label: tag }))}
            />
          )}
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
              { value: 'duplicate', label: t('todo.duplicate') },
            ]}
          />
        </div>

        <div className={`filter-control ${filters.updated_after ? 'filter-active' : ''}`}>
          <DatePicker
            showTime
            placeholder={t('filter.updatedAt')}
            allowClear
            style={{ width: 200 }}
            value={filters.updated_after ? dayjs(filters.updated_after) : null}
            onChange={(date) => {
              if (!date) {
                onChange({ updated_after: undefined, page: 1 });
              } else {
                onChange({ updated_after: date.toISOString(), page: 1 });
              }
            }}
            presets={[
              { label: t('filter.last1Day'), value: dayjs().subtract(1, 'day') },
              { label: t('filter.last3Days'), value: dayjs().subtract(3, 'day') },
              { label: t('filter.last7Days'), value: dayjs().subtract(7, 'day') },
              { label: t('filter.last30Days'), value: dayjs().subtract(30, 'day') },
            ]}
          />
        </div>

        <Button
          className="filter-clear-btn"
          size="small"
          icon={<ClearOutlined />}
          onClick={() => onChange({ tag: undefined, category: undefined, priority: undefined, status: undefined, updated_after: undefined, page: 1 })}
        >
          {t('common.clear')}
        </Button>
      </div>

      {/* Row 3: Sort controls */}
      <div className="filter-bar__row">
        <div className={`filter-control ${filters.sort_by ? 'filter-active' : ''}`}>
          <Select
            placeholder={t('filter.sortBy')}
            allowClear
            style={{ width: 140 }}
            value={filters.sort_by || undefined}
            onChange={(value: string | undefined) => onChange({ sort_by: value || undefined })}
            options={[
              { value: 'created_at', label: t('filter.created') },
              { value: 'updated_at', label: t('filter.updated') },
              { value: 'due_at', label: t('filter.dueDate') },
            ]}
          />
        </div>

        <div className={`filter-control ${filters.sort_order ? 'filter-active' : ''}`}>
          <Select
            placeholder={t('filter.desc')}
            style={{ width: 100 }}
            value={filters.sort_order || 'desc'}
            onChange={(value: 'asc' | 'desc') => onChange({ sort_order: value })}
            options={[
              { value: 'desc', label: t('filter.desc') },
              { value: 'asc', label: t('filter.asc') },
            ]}
          />
        </div>

        <Button
          className="filter-clear-btn"
          size="small"
          icon={<ClearOutlined />}
          onClick={() => onChange({ sort_by: undefined, sort_order: undefined })}
        >
          {t('common.clear')}
        </Button>
      </div>
    </div>
  );
}


