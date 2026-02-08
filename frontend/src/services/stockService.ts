// 市场数据服务 - 支持 Wails 和 Web 双模式
import { isWailsEnv, httpRequest } from './apiAdapter';
import type { Stock, KLineData, OrderBook } from '../types';

// 股票搜索结果类型
export interface StockSearchResult {
  symbol: string;
  name: string;
  industry: string;
  market: string;
}

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

export const getStockRealTimeData = async (codes: string[]): Promise<Stock[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetStockRealTimeData(codes);
  }
  return httpRequest<Stock[]>(`/api/stock/realtime?codes=${codes.join(',')}`);
};

export const getKLineData = async (code: string, period: string, days: number): Promise<KLineData[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetKLineData(code, period, days);
  }
  return httpRequest<KLineData[]>(`/api/stock/kline?code=${code}&period=${period}`);
};

// 获取真实五档盘口数据
export const getOrderBook = async (code: string): Promise<OrderBook> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetOrderBook(code);
  }
  return httpRequest<OrderBook>(`/api/stock/orderbook?code=${code}`);
};

// 搜索股票
export const searchStocks = async (keyword: string): Promise<StockSearchResult[]> => {
  if (!keyword.trim()) return [];
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.SearchStocks(keyword) as StockSearchResult[];
  }
  return httpRequest<StockSearchResult[]>(`/api/stock/search?q=${encodeURIComponent(keyword)}`);
};
