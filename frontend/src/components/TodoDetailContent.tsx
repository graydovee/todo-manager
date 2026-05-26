import { useState } from 'react';
import { Tag, Button, Space, Typography, Divider, Popconfirm, message, Descriptions, Modal } from 'antd';
import {
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  UndoOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useCompleteTodo, useDeleteTodo, useHighlightTodo, usePinTodo, useReopenTodo, useSetTodoStatus, useStartTodo } from '../hooks/useTodos';
import { TodoComments } from './TodoComments';
import { MoreActionsMenu } from './MoreActionsMenu';
import { DuplicatesList } from './DuplicatesList';
import { DuplicateSelector } from './DuplicateSelector';
import { formatDisplayCode } from '../utils/displayCode';
import type { Status, TodoDetail, TodoSummary } from '../types';
import type { HttpErrorLike, RelationConflictItem } from '../types/errors';
import './TodoDetailPanel.css';

const { Title, Text, Paragraph } = Typography;



const STATUS_LABELS: Record<string, string> = {
  open: 'todo.open',
  in_progress: 'todo.inProgress',
  completed: 'todo.completed',
  duplicate: 'todo.duplicate',
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
  const [showDuplicateSelector, setShowDuplicateSelector] = useState(false);
  const startMutation = useStartTodo();
  const setStatusMutation = useSetTodoStatus();
  const completeMutation = useCompleteTodo();
  const reopenMutation = useReopenTodo();
  const deleteMutation = useDeleteTodo();
  const pinMutation = usePinTodo();
  const highlightMutation = useHighlightTodo();

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
                  <li key={item.id}>{formatDisplayCode(item.category, item.code)} - {item.title}</li>
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
                  <li key={item.id}>{formatDisplayCode(item.category, item.code)} - {item.title}</li>
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

  const handleTogglePin = async () => {
    try {
      await pinMutation.mutateAsync({ id: todo.id, pinned: !todo.pinned });
    } catch {
      message.error(t('detail.pin'));
    }
  };

  const handleToggleHighlight = async () => {
    try {
      await highlightMutation.mutateAsync({ id: todo.id, highlighted: !todo.highlighted });
    } catch {
      message.error(t('detail.highlight'));
    }
  };

  const SummaryLink = ({ item }: { item: TodoSummary }) => (
    <Space size={4}>
      <Tag className={`detail-panel-tag-status-${item.status}`}>
        {t(STATUS_LABELS[item.status])}
      </Tag>
      <a onClick={() => onNavigate(item.id)} style={{ cursor: 'pointer' }}>
        <LinkOutlined style={{ marginRight: 4 }} />
        {formatDisplayCode(item.category, item.code)} - {item.title}
      </a>
    </Space>
  );

  return (
    <div className="detail-panel-content">
      <div
        className="detail-panel-header"
        style={{
          position: stickyHeader ? 'sticky' : 'static',
        }}
      >
        <Space wrap>
          <span className={`detail-panel-code detail-panel-code-${todo.category}`}>{formatDisplayCode(todo.category, todo.code)}</span>
          <span className="detail-panel-title">{todo.title}</span>
          <Tag className={`detail-panel-tag-status-${todo.status}`}>{t(STATUS_LABELS[todo.status])}</Tag>
          <Tag className={`detail-panel-tag-category-${todo.category}`}>{todo.category.toUpperCase()}</Tag>
          <Tag className={`detail-panel-tag-priority-${todo.priority}`}>{todo.priority.toUpperCase()}</Tag>
        </Space>
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(todo.id)}>
            {t('common.edit')}
          </Button>
          <Popconfirm title={t('confirm.deleteTodo')} onConfirm={handleDelete}>
            <Button className="detail-panel-btn-delete" size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      </div>

      <Space wrap style={{ marginBottom: 16 }}>
        {todo.status === 'open' && (
          <Button className="detail-panel-btn-start" type="primary" icon={<PlayCircleOutlined />} onClick={handleNextStage} loading={startMutation.isPending}>
            {t('detail.startProgress')}
          </Button>
        )}
        {todo.status === 'in_progress' && (
          <Button className="detail-panel-btn-complete" type="primary" icon={<CheckCircleOutlined />} onClick={handleNextStage} loading={completeMutation.isPending}>
            {t('detail.markComplete')}
          </Button>
        )}
        {todo.status === 'completed' && (
          <Button icon={<UndoOutlined />} onClick={handleReopen} loading={reopenMutation.isPending}>
            {t('detail.reopen')}
          </Button>
        )}
        <MoreActionsMenu
          todo={todo}
          onJumpToStage={handleJumpToStage}
          onAddPrerequisite={() => onAddPrerequisite(todo.id)}
          onTogglePin={handleTogglePin}
          onToggleHighlight={handleToggleHighlight}
          onMarkDuplicate={() => {
            if (todo.status !== 'duplicate') {
              setShowDuplicateSelector(true);
            }
          }}
        />
      </Space>

      {todo.description && (
        <div className="detail-panel-description">
          <Title level={5}>{t('todo.description')}</Title>
          <Paragraph>{todo.description}</Paragraph>
        </div>
      )}

      <Descriptions className="detail-panel-metadata" size="small" column={2} style={{ marginBottom: 12 }}>
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
          <div className="detail-panel-dependencies">
            {todo.depends_on.length > 0 && (
              <div style={{ marginBottom: 4 }}>
                <Text strong>{t('detail.dependsOn')}: </Text>
                <div className="detail-panel-dependencies-list">
                  {todo.depends_on.map((dep) => (
                    <div key={dep.id}><SummaryLink item={dep} /></div>
                  ))}
                </div>
              </div>
            )}
            {todo.depended_by.length > 0 && (
              <div style={{ marginBottom: 4 }}>
                <Text strong>{t('detail.dependedBy')}: </Text>
                <div className="detail-panel-dependencies-list">
                  {todo.depended_by.map((dep) => (
                    <div key={dep.id}><SummaryLink item={dep} /></div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </>
      )}

      <DuplicatesList
        duplicateOf={todo.duplicate_of}
        duplicates={todo.duplicates}
        onNavigate={onNavigate}
      />

      {showDuplicateSelector && (
        <>
          <Divider style={{ margin: '8px 0' }} />
          <DuplicateSelector
            currentTodoId={todo.id}
            onSelect={() => setShowDuplicateSelector(false)}
            onCancel={() => setShowDuplicateSelector(false)}
          />
        </>
      )}

      <Divider style={{ margin: '8px 0' }} />

      <Title level={5}>{t('detail.comments')}</Title>
      <TodoComments todoId={todo.id} />
    </div>
  );
}
