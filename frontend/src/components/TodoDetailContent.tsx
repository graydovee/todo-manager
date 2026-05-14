import { Tag, Button, Space, Typography, Divider, Dropdown, Popconfirm, message, Descriptions, Modal } from 'antd';
import {
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  UndoOutlined,
  PlusOutlined,
  DownOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useCompleteTodo, useDeleteTodo, useReopenTodo, useSetTodoStatus, useStartTodo } from '../hooks/useTodos';
import { TodoComments } from './TodoComments';
import type { Status, TodoDetail, TodoSummary } from '../types';
import type { HttpErrorLike, RelationConflictItem } from '../types/errors';

const { Title, Text, Paragraph } = Typography;

const CATEGORY_COLORS: Record<string, string> = {
  bug: 'red',
  feature: 'blue',
  task: 'green',
};

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

const STATUS_LABELS: Record<string, string> = {
  open: 'todo.open',
  in_progress: 'todo.inProgress',
  completed: 'todo.completed',
};

interface Props {
  todo: TodoDetail;
  onEdit: (id: number) => void;
  onNavigate: (id: number) => void;
  onAddPrerequisite: (todoId: number) => void;
  onDelete?: () => void;
  stickyHeader?: boolean;
}

export function TodoDetailContent({
  todo,
  onEdit,
  onNavigate,
  onAddPrerequisite,
  onDelete,
  stickyHeader = true,
}: Props) {
  const { t } = useTranslation();
  const startMutation = useStartTodo();
  const setStatusMutation = useSetTodoStatus();
  const completeMutation = useCompleteTodo();
  const reopenMutation = useReopenTodo();
  const deleteMutation = useDeleteTodo();

  const getConflictItems = (error: unknown, key: 'pending_dependencies' | 'completed_dependents'): RelationConflictItem[] => {
    const httpError = error as HttpErrorLike;
    return httpError.response?.data?.[key] || [];
  };

  const handleNextStage = async () => {
    try {
      if (todo.status === 'open') {
        await startMutation.mutateAsync(todo.id);
        message.success(t('confirm.todoStarted'));
      } else if (todo.status === 'in_progress') {
        await completeMutation.mutateAsync({ id: todo.id, cascade: false });
        message.success(t('confirm.todoCompleted'));
      }
    } catch (error: unknown) {
      const httpError = error as HttpErrorLike;
      if (httpError.response?.status === 409) {
        const pending = getConflictItems(error, 'pending_dependencies');
        Modal.confirm({
          title: t('confirm.completeWithDeps'),
          content: (
            <div>
              <p>{t('confirm.hasIncompleteDeps')}</p>
              <ul>
                {pending.map((item) => (
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

  const handleReopen = async () => {
    try {
      await reopenMutation.mutateAsync({ id: todo.id, cascade: false });
      message.success(t('confirm.todoReopened'));
    } catch (error: unknown) {
      const httpError = error as HttpErrorLike;
      if (httpError.response?.status === 409) {
        const completed = getConflictItems(error, 'completed_dependents');
        Modal.confirm({
          title: t('confirm.reopenWithDependents'),
          content: (
            <div>
              <p>{t('confirm.reopenAffects')}</p>
              <ul>
                {completed.map((item) => (
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

  const handleJumpToStage = async (status: Status) => {
    await setStatusMutation.mutateAsync({ id: todo.id, input: { status } });
  };

  const handleDelete = async () => {
    await deleteMutation.mutateAsync(todo.id);
    message.success(t('confirm.todoDeleted'));
    onDelete?.();
    onNavigate(0);
  };

  const stageMenuItems = [
    { key: 'open', label: t('todo.open') },
    { key: 'in_progress', label: t('todo.inProgress') },
    { key: 'completed', label: t('todo.completed') },
  ].filter((item) => item.key !== todo.status);

  const SummaryLink = ({ item }: { item: TodoSummary }) => (
    <a onClick={() => onNavigate(item.id)} style={{ cursor: 'pointer' }}>
      <LinkOutlined style={{ marginRight: 4 }} />
      {item.code} - {item.title}
    </a>
  );

  return (
    <div style={{ padding: '0 16px 16px 16px', height: '100%', overflow: 'auto' }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          paddingBottom: 12,
          marginBottom: 12,
          borderBottom: '1px solid #f0f0f0',
          position: stickyHeader ? 'sticky' : 'static',
          top: 0,
          background: '#fff',
          zIndex: 1,
          paddingTop: 12,
        }}
      >
        <Space wrap>
          <Text code style={{ fontSize: 13 }}>{todo.code}</Text>
          <Text strong style={{ fontSize: 15 }}>{todo.title}</Text>
          <Tag color={STATUS_COLORS[todo.status]}>{t(STATUS_LABELS[todo.status])}</Tag>
          <Tag color={CATEGORY_COLORS[todo.category]}>{todo.category.toUpperCase()}</Tag>
          <Tag color={PRIORITY_COLORS[todo.priority]}>{todo.priority.toUpperCase()}</Tag>
        </Space>
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(todo.id)}>
            {t('common.edit')}
          </Button>
          <Popconfirm title={t('confirm.deleteTodo')} onConfirm={handleDelete}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      </div>

      <Space wrap style={{ marginBottom: 16 }}>
        {todo.status === 'open' && (
          <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleNextStage} loading={startMutation.isPending}>
            {t('detail.startProgress')}
          </Button>
        )}
        {todo.status === 'in_progress' && (
          <Button type="primary" icon={<CheckCircleOutlined />} onClick={handleNextStage} loading={completeMutation.isPending}>
            {t('detail.markComplete')}
          </Button>
        )}
        {todo.status === 'completed' && (
          <Button icon={<UndoOutlined />} onClick={handleReopen} loading={reopenMutation.isPending}>
            {t('detail.reopen')}
          </Button>
        )}
        <Dropdown
          menu={{
            items: stageMenuItems.map((item) => ({
              key: item.key,
              label: item.label,
              onClick: () => handleJumpToStage(item.key as Status),
            })),
          }}
          disabled={stageMenuItems.length === 0}
        >
          <Button loading={setStatusMutation.isPending}>
            {t('detail.jumpToStage')} <DownOutlined />
          </Button>
        </Dropdown>
        <Button icon={<PlusOutlined />} onClick={() => onAddPrerequisite(todo.id)}>
          {t('detail.addPrerequisite')}
        </Button>
      </Space>

      {todo.description && (
        <>
          <Title level={5}>{t('todo.description')}</Title>
          <Paragraph>{todo.description}</Paragraph>
        </>
      )}

      <Descriptions size="small" column={2} style={{ marginBottom: 12 }}>
        <Descriptions.Item label={t('todo.dueDate')}>
          {todo.due_at ? new Date(todo.due_at).toLocaleDateString() : '-'}
        </Descriptions.Item>
        <Descriptions.Item label={t('detail.createdAt')}>
          {new Date(todo.created_at).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label={t('detail.updatedAt')}>
          {new Date(todo.updated_at).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label={t('todo.tags')}>
          {todo.tags.length > 0 ? todo.tags.map((tag) => <Tag key={tag}>{tag}</Tag>) : '-'}
        </Descriptions.Item>
      </Descriptions>

      {(todo.depends_on.length > 0 || todo.depended_by.length > 0) && (
        <>
          <Divider style={{ margin: '8px 0' }} />
          <div style={{ marginBottom: 12 }}>
            {todo.depends_on.length > 0 && (
              <div style={{ marginBottom: 4 }}>
                <Text strong>{t('detail.dependsOn')}: </Text>
                <div style={{ paddingLeft: 16 }}>
                  {todo.depends_on.map((dep) => (
                    <div key={dep.id}><SummaryLink item={dep} /></div>
                  ))}
                </div>
              </div>
            )}
            {todo.depended_by.length > 0 && (
              <div style={{ marginBottom: 4 }}>
                <Text strong>{t('detail.dependedBy')}: </Text>
                <div style={{ paddingLeft: 16 }}>
                  {todo.depended_by.map((dep) => (
                    <div key={dep.id}><SummaryLink item={dep} /></div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </>
      )}

      <Divider style={{ margin: '8px 0' }} />

      <Title level={5}>{t('detail.comments')}</Title>
      <TodoComments todoId={todo.id} />
    </div>
  );
}
