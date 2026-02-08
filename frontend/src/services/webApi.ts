// Web 版本的 API 服务封装
// 在 Docker/Web 环境中使用 HTTP API，替代 Wails 绑定

const API_BASE = '';

// 检查认证状态
export async function checkAuthStatus(): Promise<{ authRequired: boolean; authenticated: boolean }> {
  const res = await fetch(`${API_BASE}/api/auth/status`);
  return res.json();
}

// 登录
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

// 登出
export async function logout(): Promise<void> {
  await fetch(`${API_BASE}/api/auth/logout`, { method: 'POST' });
}

// 通用 API 请求封装
async function apiRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });
  
  if (res.status === 401) {
    // 未授权，触发重新登录
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
    throw new Error('Unauthorized');
  }
  
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error || 'Request failed');
  }
  
  return res.json();
}

// API 方法导出
export const webApi = {
  // 配置
  getConfig: () => apiRequest('/api/config'),
  updateConfig: (config: unknown) => apiRequest('/api/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  }),

  // 自选股
  getWatchlist: () => apiRequest('/api/watchlist'),
  addToWatchlist: (stock: unknown) => apiRequest('/api/watchlist', {
    method: 'POST',
    body: JSON.stringify(stock),
  }),
  removeFromWatchlist: (symbol: string) => apiRequest(`/api/watchlist?symbol=${symbol}`, {
    method: 'DELETE',
  }),

  // 股票数据
  getStockRealtime: (codes: string[]) => apiRequest(`/api/stock/realtime?codes=${codes.join(',')}`),
  getKLine: (code: string, period: string) => apiRequest(`/api/stock/kline?code=${code}&period=${period}`),
  getOrderBook: (code: string) => apiRequest(`/api/stock/orderbook?code=${code}`),
  searchStocks: (q: string) => apiRequest(`/api/stock/search?q=${q}`),

  // Session
  getSession: (stockCode: string, stockName: string) => 
    apiRequest(`/api/session?stockCode=${stockCode}&stockName=${stockName}`),
  getSessionMessages: (stockCode: string) => 
    apiRequest(`/api/session/messages?stockCode=${stockCode}`),
  clearSessionMessages: (stockCode: string) => 
    apiRequest(`/api/session/messages?stockCode=${stockCode}`, { method: 'DELETE' }),

  // Agents
  getAgents: () => apiRequest('/api/agents'),
  
  // 新闻
  getTelegraphList: () => apiRequest('/api/news/telegraph'),

  // 热点
  getHotTrendPlatforms: () => apiRequest('/api/hottrend/platforms'),
  getHotTrend: (platform?: string) => 
    apiRequest(platform ? `/api/hottrend?platform=${platform}` : '/api/hottrend'),

  // 工具
  getTools: () => apiRequest('/api/tools'),

  // MCP
  getMCPServers: () => apiRequest('/api/mcp/servers'),
  getMCPStatus: () => apiRequest('/api/mcp/status'),

  // 版本
  getVersion: () => apiRequest<{ version: string }>('/api/version'),
};
