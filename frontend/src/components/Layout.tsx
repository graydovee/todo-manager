import { Layout as AntLayout, Button, Typography } from 'antd';
import { LogoutOutlined, GlobalOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../stores/authStore';
import { useLang } from '../stores/langStore';
import type { ReactNode } from 'react';

const { Header, Content } = AntLayout;
const { Title } = Typography;

export function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { t } = useTranslation();
  const { lang, setLang } = useLang();

  const toggleLang = () => {
    setLang(lang === 'en' ? 'zh' : 'en');
  };

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px' }}>
        <Title level={4} style={{ color: 'white', margin: 0 }}>{t('common.appName')}</Title>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Button
            type="text"
            icon={<GlobalOutlined />}
            onClick={toggleLang}
            style={{ color: 'white' }}
          >
            {lang === 'en' ? '中文' : 'EN'}
          </Button>
          {user && (
            <>
              <span style={{ color: 'white' }}>{user.display_name}</span>
              <Button
                type="text"
                icon={<LogoutOutlined />}
                onClick={logout}
                style={{ color: 'white' }}
              >
                {t('login.logout')}
              </Button>
            </>
          )}
        </div>
      </Header>
      <Content style={{ padding: '16px', height: 'calc(100vh - 64px)', overflow: 'hidden' }}>
        {children}
      </Content>
    </AntLayout>
  );
}
