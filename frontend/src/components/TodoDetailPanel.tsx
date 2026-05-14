import { Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useTodo } from '../hooks/useTodos';
import { TodoDetailContent } from './TodoDetailContent';

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
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
        <Text type="secondary">{t('detail.noSelection')}</Text>
      </div>
    );
  }

  if (isLoading || !todo) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
        <Text type="secondary">Loading...</Text>
      </div>
    );
  }

  return (
    <TodoDetailContent todo={todo} onEdit={onEdit} onNavigate={onNavigate} onAddPrerequisite={onAddPrerequisite} onDelete={onDelete} />
  );
}
