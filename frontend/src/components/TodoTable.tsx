import { Table, Tag, Button, Popconfirm, message, Space, Modal, Tooltip } from 'antd';
import { CheckCircleOutlined, UndoOutlined, DeleteOutlined, PlayCircleOutlined, EditOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ColumnsType } from 'antd/es/table';
import type { Todo, PaginatedResponse } from '../types';
import { useStartTodo, useCompleteTodo, useReopenTodo, useDeleteTodo } from '../hooks/useTodos';

const PRIORITY_COLORS: Record<string, string> = {
  p0: 'red',
  p1: 'orange',
  p2: 'gold',
  p3: 'default',
};

const STATUS_COLORS: Record<string, string> = {
  open: 'default',
  in_progress: 'blue',
  completed: 'green',
};

const PRIORITY_WEIGHT: Record<string, number> = {
  p0: 0,
  p1: 1,
  p2: 2,
  p3: 3,
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
  const startMutation = useStartTodo();
  const completeMutation = useCompleteTodo();
  const reopenMutation = useReopenTodo();
  const deleteMutation = useDeleteTodo();

  const handleStart = async (id: number) => {
    await startMutation.mutateAsync(id);
    message.success(t('confirm.todoStarted'));
  };

  const handleComplete = async (todo: Todo) => {
    try {
      await completeMutation.mutateAsync({ id: todo.id, cascade: false });
      message.success(t('confirm.todoCompleted'));
    } catch (error: any) {
      if (error?.response?.status === 409) {
        const pending = error.response.data.pending_dependencies || [];
        Modal.confirm({
          title: t('confirm.completeWithDeps'),
          content: (
            <div>
              <p>{t('confirm.hasIncompleteDeps')}</p>
              <ul>
                {pending.map((item: any) => (
                  <li key={item.id}>{item.code} - {item.title}</li>
                ))}
              </ul>
              <p>{t('confirm.completeAllToo')}</p>
            </div>
          ),
          onOk: async () => {
            await completeMutation.mutateAsync({ id: todo.id, cascade: true });
            message.success(t('confirm.completedWithDeps'));
          },
        });
      }
    }
  };

  const handleReopen = async (todo: Todo) => {
    try {
      await reopenMutation.mutateAsync({ id: todo.id, cascade: false });
      message.success(t('confirm.todoReopened'));
    } catch (error: any) {
      if (error?.response?.status === 409) {
        const completed = error.response.data.completed_dependents || [];
        Modal.confirm({
          title: t('confirm.reopenWithDependents'),
          content: (
            <div>
              <p>{t('confirm.reopenAffects')}</p>
              <ul>
                {completed.map((item: any) => (
                  <li key={item.id}>{item.code} - {item.title}</li>
                ))}
              </ul>
              <p>{t('confirm.reopenAllToo')}</p>
            </div>
          ),
          onOk: async () => {
            await reopenMutation.mutateAsync({ id: todo.id, cascade: true });
            message.success(t('confirm.reopenedWithDependents'));
          },
        });
      }
    }
  };

  const handleDelete = async (id: number) => {
    await deleteMutation.mutateAsync(id);
    message.success(t('confirm.todoDeleted'));
  };

  const columns: ColumnsType<Todo> = [
    {
      title: t('todo.code'),
      dataIndex: 'code',
      key: 'code',
      width: 100,
      sorter: true,
      sortOrder: sortBy === 'code' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (code: string) => <span style={{ fontFamily: 'monospace', fontWeight: 'bold', fontSize: 12 }}>{code}</span>,
    },
    {
      title: t('todo.titleLabel'),
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
      sorter: true,
      sortOrder: sortBy === 'title' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
    },
    {
      title: t('todo.priority'),
      dataIndex: 'priority',
      key: 'priority',
      width: 65,
      sorter: ({ priority: a }, { priority: b }) => PRIORITY_WEIGHT[a] - PRIORITY_WEIGHT[b],
      sortOrder: sortBy === 'priority' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (p: string) => <Tag color={PRIORITY_COLORS[p]}>{p.toUpperCase()}</Tag>,
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
        return <Tag color={STATUS_COLORS[status]}>{labels[status] || status}</Tag>;
      },
    },
    {
      title: t('todo.dueDate'),
      dataIndex: 'due_at',
      key: 'due_at',
      width: 100,
      sorter: true,
      sortOrder: sortBy === 'due_at' ? (sortOrder === 'asc' ? 'ascend' : sortOrder === 'desc' ? 'descend' : null) : null,
      render: (due: string | null) => {
        if (!due) return <span style={{ color: '#ccc' }}>-</span>;
        return new Date(due).toLocaleDateString();
      },
    },
    {
      title: t('todo.actions'),
      key: 'actions',
      width: 120,
      render: (_: any, record: Todo) => (
        <Space size={0}>
          {record.status === 'open' && (
            <Tooltip title={t('detail.startProgress')}>
              <Button type="text" size="small" icon={<PlayCircleOutlined />} onClick={(e) => { e.stopPropagation(); handleStart(record.id); }} />
            </Tooltip>
          )}
          {(record.status === 'open' || record.status === 'in_progress') ? (
            <Tooltip title={t('detail.markComplete')}>
              <Button type="text" size="small" icon={<CheckCircleOutlined style={{ color: 'green' }} />} onClick={(e) => { e.stopPropagation(); handleComplete(record); }} />
            </Tooltip>
          ) : (
            <Tooltip title={t('detail.reopen')}>
              <Button type="text" size="small" icon={<UndoOutlined />} onClick={(e) => { e.stopPropagation(); handleReopen(record); }} />
            </Tooltip>
          )}
          <Tooltip title={t('common.edit')}>
            <Button type="text" size="small" icon={<EditOutlined />} onClick={(e) => { e.stopPropagation(); onEdit(record.id); }} />
          </Tooltip>
          <Popconfirm title={t('confirm.deleteTodo')} onConfirm={() => handleDelete(record.id)}>
            <Tooltip title={t('common.delete')}>
              <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={(e) => e.stopPropagation()} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
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
      rowClassName={(record) => (record.id === selectedRowId ? 'ant-table-row-selected' : '')}
      onRow={(record) => ({
        onClick: () => onSelect(record.id),
        onDoubleClick: () => onEdit(record.id),
        style: { cursor: 'pointer' },
      })}
    />
  );
}
