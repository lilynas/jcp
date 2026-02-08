// 自选股服务 - 支持 Wails 和 Web 双模式
import { isWailsEnv, httpRequest } from './apiAdapter';
import type { Stock } from '../types';

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

export const getWatchlist = async (): Promise<Stock[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetWatchlist() as Stock[];
  }
  return httpRequest<Stock[]>('/api/watchlist');
};

export const addToWatchlist = async (stock: Stock): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.AddToWatchlist(stock as any);
  }
  await httpRequest('/api/watchlist', {
    method: 'POST',
    body: JSON.stringify(stock),
  });
  return 'success';
};

export const removeFromWatchlist = async (symbol: string): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.RemoveFromWatchlist(symbol);
  }
  await httpRequest(`/api/watchlist?symbol=${symbol}`, {
    method: 'DELETE',
  });
  return 'success';
};
