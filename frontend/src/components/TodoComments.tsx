import { useState } from 'react';
import { Input, Button, List, Popconfirm, Typography, Space } from 'antd';
import { DeleteOutlined, SendOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useComments, useCreateComment, useDeleteComment } from '../hooks/useTodos';

const { Text } = Typography;

interface Props {
  todoId: number;
}

export function TodoComments({ todoId }: Props) {
  const { t } = useTranslation();
  const { data: comments, isLoading } = useComments(todoId);
  const createMutation = useCreateComment();
  const deleteMutation = useDeleteComment();
  const [content, setContent] = useState('');

  const handleSubmit = async () => {
    if (!content.trim()) return;
    await createMutation.mutateAsync({ todoId, content: content.trim() });
    setContent('');
  };

  return (
    <div>
      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <Input
          placeholder={t('detail.commentPlaceholder')}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          onPressEnter={handleSubmit}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          loading={createMutation.isPending}
          onClick={handleSubmit}
        >
          {t('detail.submitComment')}
        </Button>
      </div>
      <List
        loading={isLoading}
        dataSource={comments || []}
        locale={{ emptyText: t('detail.noComments') }}
        renderItem={(comment) => (
          <List.Item
            actions={[
              <Popconfirm
                key="delete"
                title={t('confirm.deleteCommentConfirm')}
                onConfirm={() => deleteMutation.mutateAsync({ todoId, commentId: comment.id })}
              >
                <Button size="small" danger icon={<DeleteOutlined />} />
              </Popconfirm>,
            ]}
          >
            <List.Item.Meta
              description={
                <Space direction="vertical" size={0}>
                  <Text>{comment.content}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {new Date(comment.created_at).toLocaleString()}
                  </Text>
                </Space>
              }
            />
          </List.Item>
        )}
      />
    </div>
  );
}
