import { Layout as AntLayout, Button, Menu, Typography } from 'antd';
import { LogoutOutlined, GlobalOutlined, MenuFoldOutlined, MenuUnfoldOutlined, ApartmentOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../stores/authContext';
import { useLang } from '../stores/langStore';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useEffect, useMemo, useState } from 'react';
import './Layout.css';

const { Header, Content, Sider } = AntLayout;
const { Title } = Typography;

const SIDEBAR_COLLAPSED_KEY = 'app.sidebar.collapsed';

export function Layout() {
  const { user, logout } = useAuth();
  const { t } = useTranslation();
  const { lang, setLang } = useLang();
  const location = useLocation();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1');

  const toggleLang = () => {
    setLang(lang === 'en' ? 'zh' : 'en');
  };

  useEffect(() => {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  const selectedKey = useMemo(() => {
    if (location.pathname.startsWith('/todos/graph')) return '/todos/graph';
    return '/todos';
  }, [location.pathname]);

  return (
    <AntLayout className="app-shell">
      <Header className="app-header">
        <div className="app-brand">
          <div className="app-brand-mark" />
          <Title level={4} className="app-brand-title">{t('common.appName')}</Title>
        </div>
        <div className="app-header-actions">
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
              <span className="app-header-user">{user.display_name}</span>
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
      <AntLayout hasSider>
        <Sider className="app-sider" width={240} collapsedWidth={84} collapsed={collapsed} trigger={null}>
          <div className="app-sider-top">
            <Button
              type="text"
              className="app-collapse-btn"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={() => setCollapsed((prev) => !prev)}
            />
          </div>
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            className="app-menu"
            items={[
              {
                key: '/todos',
                icon: <UnorderedListOutlined />,
                label: t('nav.todos'),
                onClick: () => navigate('/todos'),
              },
              {
                key: '/todos/graph',
                icon: <ApartmentOutlined />,
                label: t('nav.graph'),
                onClick: () => navigate('/todos/graph'),
              },
            ]}
          />
        </Sider>
        <Content className="app-content">
          <div className="app-content-body"><Outlet /></div>
        </Content>
      </AntLayout>
    </AntLayout>
  );
}
