import { Table, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ColumnsType } from 'antd/es/table';
import type { Todo, PaginatedResponse } from '../types';
import { formatDisplayCode } from '../utils/displayCode';

const PRIORITY_COLORS: Record<string, string> = {
  p0: 'tag-priority-p0',
  p1: 'tag-priority-p1',
  p2: 'tag-priority-p2',
  p3: 'tag-priority-p3',
};

const STATUS_COLORS: Record<string, string> = {
  open: 'tag-status-open',
  in_progress: 'tag-status-in_progress',
  completed: 'tag-status-completed',
};

const PRIORITY_WEIGHT: Record<string, number> = {
  p0: 0,
  p1: 1,
  p2: 2,
  p3: 3,
};

const CATEGORY_COLORS: Record<string, string> = {
  bug: 'var(--color-category-bug)',
  feature: 'var(--color-category-feature)',
  task: 'var(--color-category-task)',
};

interface Props {
  data?: PaginatedResponse<Todo>;
  loading: boolean;
  selectedRowId: number | null;
  onSelect: (id: number) => void;
  onEdit: (id: number) => void;
  onPageChange: (page: number, pageSize: number) => void;
  onSortChange: (field: string | undefined, order: 'asc' | 'desc' | undefined) => void;
  page: number;
  pageSize: number;
  sortBy?: string;
  sortOrder?: 'asc' | 'desc';
}

export function TodoTable({ data, loading, selectedRowId, onSelect, onEdit, onPageChange, onSortChange, page, pageSize, sortBy, sortOrder }: Props) {
  const { t } = useTranslation();

  const columns: ColumnsType<Todo> = [
    {
      title: t('todo.titleLabel'),
      key: 'title',
      ellipsis: true,
      sorter: true,
      sortOrder: sortBy === 'title' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (_: unknown, record: Todo) => (
        <span>
          <span className="todo-code-badge" style={{ color: CATEGORY_COLORS[record.category] || 'inherit' }}>{formatDisplayCode(record.category, record.code)}</span>
          {record.title}
        </span>
      ),
    },
    {
      title: t('todo.priority'),
      dataIndex: 'priority',
      key: 'priority',
      width: 75,
      sorter: ({ priority: a }, { priority: b }) => PRIORITY_WEIGHT[a] - PRIORITY_WEIGHT[b],
      sortOrder: sortBy === 'priority' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (p: string) => <Tag className={PRIORITY_COLORS[p]}>{p.toUpperCase()}</Tag>,
    },
    {
      title: t('todo.status'),
      dataIndex: 'status',
      key: 'status',
      width: 95,
      sorter: true,
      sortOrder: sortBy === 'status' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (status: string) => {
        const labels: Record<string, string> = {
          open: t('todo.open'),
          in_progress: t('todo.inProgress'),
          completed: t('todo.completed'),
        };
        return <Tag className={STATUS_COLORS[status]}>{labels[status] || status}</Tag>;
      },
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={data?.items || []}
      rowKey="id"
      loading={loading}
      size="small"
      pagination={{
        current: page,
        pageSize,
        total: data?.total || 0,
        showSizeChanger: true,
        showTotal: (total) => t('common.items', { count: total }),
        onChange: onPageChange,
        size: 'small',
      }}
      onChange={(_pagination, _filters, sorter) => {
        if (!Array.isArray(sorter) && sorter.column) {
          const field = sorter.field as string;
          const order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : undefined;
          onSortChange(field, order);
        } else {
          onSortChange(undefined, undefined);
        }
      }}
      rowClassName={(record) => {
        const classes: string[] = [];
        if (record.id === selectedRowId) classes.push('ant-table-row-selected');
        if (record.status === 'completed') classes.push('row-completed');
        return classes.join(' ');
      }}
      onRow={(record) => ({
        onClick: () => onSelect(record.id),
        onDoubleClick: () => onEdit(record.id),
        style: { cursor: 'pointer' },
      })}
    />
  );
}
