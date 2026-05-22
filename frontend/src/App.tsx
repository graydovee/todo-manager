import { useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import { AuthProvider } from './stores/authStore';
import { LangProvider } from './stores/langStore';
import { Layout } from './components/Layout';
import { AuthGuard } from './components/AuthGuard';
import { LoginPage } from './pages/LoginPage';
import { TodoListPage } from './pages/TodoListPage';
import { TodoGraphPage } from './pages/TodoGraphPage';
import { AISummaryPage } from './pages/AISummaryPage';
import { AISummaryResultPage } from './pages/AISummaryResultPage';
import i18n from './i18n';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

const antLocales: Record<string, typeof enUS> = { en: enUS, zh: zhCN };

function getLang(): string {
  return localStorage.getItem('lang') || (navigator.language.startsWith('zh') ? 'zh' : 'en');
}

function App() {
  const [lang, setLangState] = useState(getLang);

  const setLang = (l: string) => {
    localStorage.setItem('lang', l);
    i18n.changeLanguage(l);
    setLangState(l);
  };

  return (
    <ConfigProvider
      locale={antLocales[lang] || enUS}
      theme={{
        token: {
          colorPrimary: '#1e88a8',
          colorSuccess: '#10b981',
          colorWarning: '#f59e0b',
          colorInfo: '#3b82f6',
          colorError: '#dc2626',
          colorBgContainer: '#ffffff',
          colorBgElevated: '#ffffff',
          colorTextBase: '#0b233b',
          colorTextSecondary: '#5f7184',
          borderRadius: 14,
          fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
          fontSize: 14,
        },
        components: {
          Button: {
            borderRadius: 14,
            controlHeight: 36,
          },
          Tag: {
            borderRadiusSM: 999,
          },
          Card: {
            borderRadiusLG: 24,
          },
          Segmented: {
            trackPadding: 4,
          },
          Select: {
            borderRadius: 14,
            controlHeight: 36,
          },
        },
      }}
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthProvider>
            <LangProvider value={{ lang, setLang }}>
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route
                  path="/"
                  element={
                    <AuthGuard>
                      <Layout />
                    </AuthGuard>
                  }
                >
                  <Route index element={<Navigate to="/todos" replace />} />
                  <Route path="todos" element={<TodoListPage />} />
                  <Route path="todos/graph" element={<TodoGraphPage />} />
                  <Route path="ai-summary" element={<AISummaryPage />} />
                  <Route path="ai-summary/:id" element={<AISummaryResultPage />} />
                </Route>
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </LangProvider>
          </AuthProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

export default App;
