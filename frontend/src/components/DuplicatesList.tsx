import { Tag, Space, Typography, Divider } from 'antd';
import { LinkOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { formatDisplayCode } from '../utils/displayCode';
import type { TodoSummary } from '../types';

const { Text } = Typography;

const STATUS_LABELS: Record<string, string> = {
  open: 'todo.open',
  in_progress: 'todo.inProgress',
  completed: 'todo.completed',
  duplicate: 'todo.duplicate',
};

interface DuplicatesListProps {
  duplicateOf: TodoSummary | null;
  duplicates: TodoSummary[];
  onNavigate: (id: number) => void;
}

export function DuplicatesList({ duplicateOf, duplicates, onNavigate }: DuplicatesListProps) {
  const { t } = useTranslation();

  // Hide section entirely when no duplicate relations exist
  if (!duplicateOf && duplicates.length === 0) {
    return null;
  }

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
    <>
      <Divider style={{ margin: '8px 0' }} />
      <div className="detail-panel-dependencies">
        {duplicateOf && (
          <div style={{ marginBottom: 4 }}>
            <Text strong>{t('detail.duplicateOf')}: </Text>
            <div className="detail-panel-dependencies-list">
              <div><SummaryLink item={duplicateOf} /></div>
            </div>
          </div>
        )}
        {duplicates.length > 0 && (
          <div style={{ marginBottom: 4 }}>
            <Text strong>{t('detail.duplicates')}: </Text>
            <div className="detail-panel-dependencies-list">
              {duplicates.map((dup) => (
                <div key={dup.id}><SummaryLink item={dup} /></div>
              ))}
            </div>
          </div>
        )}
      </div>
    </>
  );
}
