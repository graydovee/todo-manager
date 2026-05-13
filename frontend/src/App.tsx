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
import i18n from './i18n';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

const antLocales: Record<string, any> = { en: enUS, zh: zhCN };

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
    <ConfigProvider locale={antLocales[lang] || enUS}>
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
                      <Layout>
                        <TodoListPage />
                      </Layout>
                    </AuthGuard>
                  }
                />
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
