import { Layout as AntLayout, Button, Drawer, Menu, Typography } from 'antd';
import { LogoutOutlined, GlobalOutlined, MenuFoldOutlined, MenuUnfoldOutlined, MenuOutlined, ApartmentOutlined, UnorderedListOutlined, RobotOutlined, KeyOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../stores/authContext';
import { useLang } from '../stores/langStore';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useEffect, useMemo, useState } from 'react';
import './Layout.css';

const { Header, Content, Sider } = AntLayout;
const { Title } = Typography;

const SIDEBAR_COLLAPSED_KEY = 'app.sidebar.collapsed';

function useIsMobile() {
  const [isMobile, setIsMobile] = useState(() => window.innerWidth <= 768);
  useEffect(() => {
    const handler = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  }, []);
  return isMobile;
}

export function Layout() {
  const { user, logout } = useAuth();
  const { t } = useTranslation();
  const { lang, setLang } = useLang();
  const location = useLocation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const toggleLang = () => {
    setLang(lang === 'en' ? 'zh' : 'en');
  };

  useEffect(() => {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  const selectedKey = useMemo(() => {
    if (location.pathname.startsWith('/access-keys')) return '/access-keys';
    if (location.pathname.startsWith('/ai-summary')) return '/ai-summary';
    if (location.pathname.startsWith('/todos/graph')) return '/todos/graph';
    return '/todos';
  }, [location.pathname]);

  const menuItems = [
    {
      key: '/todos',
      icon: <UnorderedListOutlined />,
      label: t('nav.todos'),
      onClick: () => { navigate('/todos'); setMobileMenuOpen(false); },
    },
    {
      key: '/todos/graph',
      icon: <ApartmentOutlined />,
      label: t('nav.graph'),
      onClick: () => { navigate('/todos/graph'); setMobileMenuOpen(false); },
    },
    {
      key: '/ai-summary',
      icon: <RobotOutlined />,
      label: t('nav.aiSummary'),
      onClick: () => { navigate('/ai-summary'); setMobileMenuOpen(false); },
    },
    {
      key: '/access-keys',
      icon: <KeyOutlined />,
      label: t('nav.accessKeys'),
      onClick: () => { navigate('/access-keys'); setMobileMenuOpen(false); },
    },
  ];

  return (
    <AntLayout className="app-shell">
      <Header className="app-header">
        <div className="app-brand">
          {isMobile && (
            <Button
              type="text"
              icon={<MenuOutlined />}
              onClick={() => setMobileMenuOpen(true)}
              className="app-mobile-menu-btn"
            />
          )}
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
              {!isMobile && <span className="app-header-user">{user.display_name}</span>}
              <Button
                type="text"
                icon={<LogoutOutlined />}
                onClick={logout}
                style={{ color: 'white' }}
              >
                {!isMobile && t('login.logout')}
              </Button>
            </>
          )}
        </div>
      </Header>
      <AntLayout hasSider={!isMobile}>
        {!isMobile && (
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
              items={menuItems}
            />
          </Sider>
        )}
        <Content className="app-content">
          <div className="app-content-body"><Outlet /></div>
        </Content>
      </AntLayout>

      {isMobile && (
        <Drawer
          title={t('common.appName')}
          placement="left"
          width={260}
          open={mobileMenuOpen}
          onClose={() => setMobileMenuOpen(false)}
          className="app-mobile-drawer"
          styles={{ body: { padding: 0 } }}
        >
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            className="app-menu"
            items={menuItems}
          />
          {user && (
            <div className="app-mobile-drawer-footer">
              <span>{user.display_name}</span>
              <Button type="text" icon={<LogoutOutlined />} onClick={logout}>
                {t('login.logout')}
              </Button>
            </div>
          )}
        </Drawer>
      )}
    </AntLayout>
  );
}
