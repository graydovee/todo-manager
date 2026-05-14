import { useState, useEffect, type ReactNode } from 'react';
import { getMe, login as apiLogin, logout as apiLogout } from '../api/auth';
import { fetchCSRFToken } from '../api/client';
import { AuthContext } from './authContext';
import type { User } from '../types';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function init() {
      try {
        await fetchCSRFToken();
        const me = await getMe();
        setUser(me);
      } catch {
        setUser(null);
      } finally {
        setLoading(false);
      }
    }
    init();
  }, []);

  const login = async (username: string, password: string) => {
    await fetchCSRFToken();
    const u = await apiLogin(username, password);
    setUser(u);
  };

  const logout = async () => {
    await apiLogout();
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
