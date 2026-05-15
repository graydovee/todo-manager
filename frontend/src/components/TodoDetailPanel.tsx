import { Spin, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useTodo } from '../hooks/useTodos';
import { TodoDetailContent } from './TodoDetailContent';
import './TodoDetailPanel.css';

const { Text } = Typography;

interface Props {
  todoId: number | null;
  onEdit: (id: number) => void;
  onNavigate: (id: number) => void;
  onAddPrerequisite: (todoId: number) => void;
  onDelete?: () => void;
}

export function TodoDetailPanel({ todoId, onEdit, onNavigate, onAddPrerequisite, onDelete }: Props) {
  const { t } = useTranslation();
  const { data: todo, isLoading } = useTodo(todoId || 0);

  if (!todoId) {
    return (
      <div className="detail-panel">
        <div className="detail-panel-empty">
          <Text type="secondary">{t('detail.noSelection')}</Text>
        </div>
      </div>
    );
  }

  if (isLoading || !todo) {
    return (
      <div className="detail-panel">
        <div className="detail-panel-loading">
          <Spin />
        </div>
      </div>
    );
  }

  return (
    <div className="detail-panel">
      <TodoDetailContent todo={todo} onEdit={onEdit} onNavigate={onNavigate} onAddPrerequisite={onAddPrerequisite} onDelete={onDelete} />
    </div>
  );
}
