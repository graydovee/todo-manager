import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Form, Input, Button, Card, Typography, message } from 'antd';
import { UserOutlined, LockOutlined, GlobalOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../stores/authContext';
import { useLang } from '../stores/langStore';
import { getAuthMode } from '../api/auth';
import type { AuthMode } from '../types';
import './LoginPage.css';

export function LoginPage() {
  const { login, user } = useAuth();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { lang, setLang } = useLang();
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<AuthMode | null>(null);

  useEffect(() => {
    getAuthMode().then(setMode).catch(() => setMode({ mode: 'basic' }));
  }, []);

  useEffect(() => {
    if (user) navigate('/', { replace: true });
  }, [user, navigate]);

  const handleSubmit = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      await login(values.username, values.password);
      message.success(t('login.loginSuccessful'));
      navigate('/');
    } catch {
      message.error(t('login.invalidCredentials'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <Card>
        <div className="login-page__header">
          <Typography.Title level={3}>
            {t('login.title')}
          </Typography.Title>
          <Button
            type="text"
            icon={<GlobalOutlined />}
            onClick={() => setLang(lang === 'en' ? 'zh' : 'en')}
          >
            {lang === 'en' ? '中文' : 'EN'}
          </Button>
        </div>

        {mode?.mode === 'basic' && (
          <Form onFinish={handleSubmit}>
            <Form.Item name="username" rules={[{ required: true, message: t('login.pleaseEnterUsername') }]}>
              <Input prefix={<UserOutlined />} placeholder={t('login.username')} size="large" />
            </Form.Item>
            <Form.Item name="password" rules={[{ required: true, message: t('login.pleaseEnterPassword') }]}>
              <Input.Password prefix={<LockOutlined />} placeholder={t('login.password')} size="large" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={loading} block size="large">
                {t('login.login')}
              </Button>
            </Form.Item>
          </Form>
        )}

        {mode?.mode === 'oidc' && (
          <Button type="primary" href="/api/v1/auth/login" block size="large">
            {t('login.loginWithSSO')}
          </Button>
        )}
      </Card>
    </div>
  );
}
