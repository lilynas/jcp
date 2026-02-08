// 环境检测与 API 适配
// 自动选择 Wails 绑定或 HTTP API

const API_BASE = '';

// 检测是否在 Wails 环境中运行
export const isWailsEnv = (): boolean => {
  return typeof (window as any).go !== 'undefined';
};

// 通用 HTTP 请求封装
export async function httpRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });
  
  if (res.status === 401) {
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
    throw new Error('Unauthorized');
  }
  
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error || 'Request failed');
  }
  
  return res.json();
}

// 检查认证状态 (仅 Web 模式需要)
export async function checkAuthStatus(): Promise<{ authRequired: boolean; authenticated: boolean }> {
  const res = await fetch(`${API_BASE}/api/auth/status`);
  return res.json();
}

// 登录 (仅 Web 模式需要)
export async function login(password: string): Promise<{ success: boolean; error?: string }> {
  const res = await fetch(`${API_BASE}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  const data = await res.json();
  if (!res.ok) {
    return { success: false, error: data.error || '登录失败' };
  }
  return { success: true };
}

// 登出 (仅 Web 模式需要)
export async function logout(): Promise<void> {
  await fetch(`${API_BASE}/api/auth/logout`, { method: 'POST' });
}
