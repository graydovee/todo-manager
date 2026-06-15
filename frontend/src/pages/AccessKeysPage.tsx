import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Checkbox, DatePicker, Empty, Form, Input, Modal, Space, Spin, Table, Tag, Typography, message } from 'antd';
import { DeleteOutlined, KeyOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { createAccessKey, deleteAccessKey, getAccessKeyPermissionCatalog, listAccessKeys, rotateAccessKey } from '../api/accessKeys';
import type { AccessKey, AccessKeyPermissionCatalog, AccessKeyGroup, RotateAccessKeyResponse } from '../types';
import './AccessKeysPage.css';

const { Paragraph, Text } = Typography;

type PresetKey = 'read' | 'write' | 'read_write' | 'summary';

function getGroupDisplay(group: AccessKeyGroup, t: (key: string, options?: Record<string, unknown>) => string) {
  const key = `accessKeys.groups.${group.id}`;
  const descriptionKey = `accessKeys.groupDescriptions.${group.id}`;
  const translatedLabel = t(key);
  const translatedDescription = t(descriptionKey);
  return {
    label: translatedLabel === key ? group.label : translatedLabel,
    description: translatedDescription === descriptionKey ? group.description : translatedDescription,
  };
}

function getAPIDisplay(api: AccessKeyPermissionCatalog['apis'][number], t: (key: string, options?: Record<string, unknown>) => string) {
  const labelKey = `accessKeys.apiLabels.${api.id}`;
  const descriptionKey = `accessKeys.apiDescriptions.${api.id}`;
  const translatedLabel = t(labelKey);
  const translatedDescription = t(descriptionKey);
  return {
    label: translatedLabel === labelKey ? api.label : translatedLabel,
    description: translatedDescription === descriptionKey ? api.description : translatedDescription,
  };
}

function summarizeAPIs(key: AccessKey, catalog: AccessKeyPermissionCatalog | null) {
  if (!catalog) return `${key.authorized_apis.length} APIs`;
  const groups = catalog.groups.filter((group) => group.permission_ids.every((id) => key.authorized_apis.includes(id)));
  if (groups.length > 0 && groups.every((group) => group.permission_ids.length > 0)) {
    return groups.map((group) => group.label).join(', ');
  }
  return `${key.authorized_apis.length} APIs`;
}

export function AccessKeysPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [rotatingId, setRotatingId] = useState<number | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [reveal, setReveal] = useState<RotateAccessKeyResponse | null>(null);
  const [keys, setKeys] = useState<AccessKey[]>([]);
  const [catalog, setCatalog] = useState<AccessKeyPermissionCatalog | null>(null);
  const [selectedAPIs, setSelectedAPIs] = useState<string[]>([]);
  const [form] = Form.useForm<{ name: string; expires_at?: dayjs.Dayjs }>();

  useEffect(() => {
    void (async () => {
      try {
        const [keyList, permissionCatalog] = await Promise.all([listAccessKeys(), getAccessKeyPermissionCatalog()]);
        setKeys(keyList);
        setCatalog(permissionCatalog);
      } catch {
        message.error(t('accessKeys.loadError'));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const groupedCatalog = useMemo(() => {
    if (!catalog) return [];
    return catalog.groups.map((group) => ({
      ...group,
      ...getGroupDisplay(group, t),
      apis: catalog.apis.filter((api) => api.group_id === group.id),
    }));
  }, [catalog, t]);

  const applyPreset = (preset: PresetKey) => {
    if (!catalog) return;
    setSelectedAPIs(catalog.presets[preset] || []);
  };

  const toggleAPI = (apiID: string, checked: boolean) => {
    setSelectedAPIs((prev) => {
      const next = new Set(prev);
      if (checked) next.add(apiID);
      else next.delete(apiID);
      return Array.from(next).sort();
    });
  };

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      const resp = await createAccessKey({
        name: values.name,
        authorized_apis: selectedAPIs,
        expires_at: values.expires_at ? values.expires_at.toISOString() : undefined,
      });
      setKeys((prev) => [resp, ...prev]);
      setReveal(resp);
      setCreateOpen(false);
      form.resetFields();
      setSelectedAPIs([]);
    } catch (error) {
      if (error instanceof Error) {
        message.error(error.message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleRotate = (key: AccessKey) => {
    Modal.confirm({
      title: t('accessKeys.rotateConfirmTitle'),
      content: t('accessKeys.rotateConfirmContent'),
      okText: t('accessKeys.rotate'),
      onOk: async () => {
        setRotatingId(key.id);
        try {
          const resp = await rotateAccessKey(key.id);
          setKeys((prev) => prev.map((entry) => (entry.id === key.id ? resp : entry)));
          setReveal(resp);
        } catch {
          message.error(t('accessKeys.rotateError'));
        } finally {
          setRotatingId(null);
        }
      },
    });
  };

  const handleDelete = (key: AccessKey) => {
    Modal.confirm({
      title: t('accessKeys.deleteConfirmTitle'),
      content: t('accessKeys.deleteConfirmContent'),
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteAccessKey(key.id);
          setKeys((prev) => prev.filter((entry) => entry.id !== key.id));
        } catch {
          message.error(t('accessKeys.deleteError'));
        }
      },
    });
  };

  const columns = [
    {
      title: t('accessKeys.name'),
      dataIndex: 'name',
      key: 'name',
      render: (value: string, record: AccessKey) => (
        <div className="access-keys-page__name-cell">
          <Text strong>{value}</Text>
          <Text type="secondary" code>{record.key_prefix}</Text>
        </div>
      ),
    },
    {
      title: t('accessKeys.permissions'),
      key: 'authorized_apis',
      render: (_: unknown, record: AccessKey) => <span>{summarizeAPIs(record, catalog)}</span>,
    },
    {
      title: t('accessKeys.expiresAt'),
      dataIndex: 'expires_at',
      key: 'expires_at',
      render: (value: string | null) => value ? dayjs(value).format('YYYY-MM-DD HH:mm') : t('accessKeys.never'),
    },
    {
      title: t('accessKeys.lastUsedAt'),
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      render: (value: string | null) => value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: t('accessKeys.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('todo.actions'),
      key: 'actions',
      render: (_: unknown, record: AccessKey) => (
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => handleRotate(record)}
            loading={rotatingId === record.id}
          >
            {t('accessKeys.rotate')}
          </Button>
          <Button danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)}>
            {t('common.delete')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="access-keys-page">
      <div className="access-keys-page__header">
        <div>
          <h2>{t('accessKeys.title')}</h2>
          <Paragraph type="secondary">{t('accessKeys.subtitle')}</Paragraph>
        </div>
        <Button type="primary" icon={<KeyOutlined />} onClick={() => setCreateOpen(true)}>
          {t('accessKeys.create')}
        </Button>
      </div>

      <Alert
        className="access-keys-page__notice"
        type="warning"
        showIcon
        title={t('accessKeys.noticeTitle')}
        description={t('accessKeys.noticeBody')}
      />

      {loading ? (
        <div className="access-keys-page__empty"><Spin size="large" /></div>
      ) : keys.length === 0 ? (
        <div className="access-keys-page__empty"><Empty description={t('accessKeys.empty')} /></div>
      ) : (
        <Table rowKey="id" columns={columns} dataSource={keys} pagination={false} />
      )}

      <Modal
        title={t('accessKeys.createTitle')}
        open={createOpen}
        onOk={() => void handleCreate()}
        onCancel={() => {
          setCreateOpen(false);
          form.resetFields();
          setSelectedAPIs([]);
        }}
        confirmLoading={submitting}
        width={880}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={t('accessKeys.name')} name="name" rules={[{ required: true, message: t('accessKeys.nameRequired') }]}>
            <Input maxLength={64} />
          </Form.Item>

          <Form.Item label={t('accessKeys.expiresAt')} name="expires_at">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>

          <div className="access-keys-page__preset-row">
            <Button onClick={() => applyPreset('read')}>{t('accessKeys.presets.read')}</Button>
            <Button onClick={() => applyPreset('write')}>{t('accessKeys.presets.write')}</Button>
            <Button onClick={() => applyPreset('read_write')}>{t('accessKeys.presets.readWrite')}</Button>
            <Button onClick={() => applyPreset('summary')}>{t('accessKeys.presets.summary')}</Button>
          </div>

          {groupedCatalog.map((group: AccessKeyGroup & { apis: AccessKeyPermissionCatalog['apis'] }) => (
            <div key={group.id} className="access-keys-page__group">
              <div className="access-keys-page__group-header">
                <Text strong>{group.label}</Text>
                <Text type="secondary">{group.description}</Text>
              </div>
              <div className="access-keys-page__api-grid">
                {group.apis.map((api) => {
                  const display = getAPIDisplay(api, t);
                  return (
                  <label key={api.id} className="access-keys-page__api-card">
                    <Checkbox
                      checked={selectedAPIs.includes(api.id)}
                      onChange={(e) => toggleAPI(api.id, e.target.checked)}
                    />
                    <div>
                      <div className="access-keys-page__api-title">
                        <Tag>{api.method}</Tag>
                        <Text strong>{display.label}</Text>
                      </div>
                      <Text code>{api.path_pattern}</Text>
                      <div><Text type="secondary">{display.description}</Text></div>
                    </div>
                  </label>
                )})}
              </div>
            </div>
          ))}
        </Form>
      </Modal>

      <Modal
        title={t('accessKeys.revealTitle')}
        open={!!reveal}
        onOk={() => setReveal(null)}
        onCancel={() => setReveal(null)}
        footer={[
          <Button key="ok" type="primary" onClick={() => setReveal(null)}>
            {t('accessKeys.close')}
          </Button>,
        ]}
      >
        <Alert
          type="warning"
          showIcon
          title={t('accessKeys.revealNotice')}
          description={t('accessKeys.revealDescription')}
        />
        <div className="access-keys-page__reveal-box">
          <Text code copyable>{reveal?.plain_key}</Text>
        </div>
      </Modal>
    </div>
  );
}
