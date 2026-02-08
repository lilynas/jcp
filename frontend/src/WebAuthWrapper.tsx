import React, { useState, useEffect } from 'react';
import { LoginPage } from './components/LoginPage';
import { isWailsEnv, checkAuthStatus, login } from './services/apiAdapter';

interface AuthState {
  loading: boolean;
  authRequired: boolean;
  authenticated: boolean;
}

// Web 版本的认证包装器
export const WebAuthWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [authState, setAuthState] = useState<AuthState>({
    loading: true,
    authRequired: false,
    authenticated: false,
  });

  // 检查认证状态
  const checkAuth = async () => {
    try {
      const status = await checkAuthStatus();
      setAuthState({
        loading: false,
        authRequired: status.authRequired,
        authenticated: status.authenticated,
      });
    } catch (err) {
      console.error('Failed to check auth status:', err);
      // 如果检查失败，假设不需要认证
      setAuthState({
        loading: false,
        authRequired: false,
        authenticated: true,
      });
    }
  };

  useEffect(() => {
    // 如果是 Wails 环境，跳过认证检查
    if (isWailsEnv()) {
      setAuthState({
        loading: false,
        authRequired: false,
        authenticated: true,
      });
      return;
    }

    checkAuth();

    // 监听未授权事件
    const handleUnauthorized = () => {
      setAuthState(prev => ({
        ...prev,
        authenticated: false,
      }));
    };

    window.addEventListener('auth:unauthorized', handleUnauthorized);
    return () => window.removeEventListener('auth:unauthorized', handleUnauthorized);
  }, []);

  // 处理登录
  const handleLogin = async (password: string) => {
    const result = await login(password);
    if (result.success) {
      setAuthState(prev => ({
        ...prev,
        authenticated: true,
      }));
    }
    return result;
  };

  // 加载中
  if (authState.loading) {
    return (
      <div className="min-h-screen fin-app flex items-center justify-center">
        <div className="text-white text-lg">加载中...</div>
      </div>
    );
  }

  // 需要登录
  if (authState.authRequired && !authState.authenticated) {
    return <LoginPage onLogin={handleLogin} />;
  }

  // 已认证或不需要认证
  return <>{children}</>;
};
