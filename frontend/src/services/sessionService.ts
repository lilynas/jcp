import { isWailsEnv, httpRequest } from './apiAdapter';
import type { StockPosition } from '../types';

export interface StockSession {
  id: string;
  stockCode: string;
  stockName: string;
  messages: ChatMessage[];
  position?: StockPosition; // 持仓信息
  createdAt: number;
  updatedAt: number;
}

export interface ChatMessage {
  id: string;
  agentId: string;
  agentName: string;
  role: string;
  content: string;
  timestamp: number;
  replyTo?: string;
  mentions?: string[];
  round?: number;
  msgType?: string;
}

// 会议室消息请求
export interface MeetingMessageRequest {
  stockCode: string;
  content: string;
  mentionIds: string[];
  replyToId: string;
  replyContent: string;
}

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

// 获取或创建Session
export const getOrCreateSession = async (stockCode: string, stockName: string): Promise<StockSession> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetOrCreateSession(stockCode, stockName);
  }
  return httpRequest<StockSession>(`/api/session?stockCode=${stockCode}&stockName=${encodeURIComponent(stockName)}`);
};

// 获取Session消息
export const getSessionMessages = async (stockCode: string): Promise<ChatMessage[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetSessionMessages(stockCode);
  }
  return httpRequest<ChatMessage[]>(`/api/session/messages?stockCode=${stockCode}`);
};

// 清空Session消息
export const clearSessionMessages = async (stockCode: string): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.ClearSessionMessages(stockCode);
  }
  await httpRequest(`/api/session/messages?stockCode=${stockCode}`, { method: 'DELETE' });
  return 'success';
};

// 发送会议室消息（@指定成员回复）
// 注意：Web 模式下此功能暂不支持（需要 WebSocket 实现实时推送）
export const sendMeetingMessage = async (req: MeetingMessageRequest): Promise<ChatMessage[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.SendMeetingMessage(req);
  }
  // Web 模式暂不支持会议室实时对话
  console.warn('sendMeetingMessage is not supported in web mode yet');
  return [];
};

// 更新股票持仓信息
export const updateStockPosition = async (stockCode: string, shares: number, costPrice: number): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.UpdateStockPosition(stockCode, shares, costPrice);
  }
  // Web 模式下暂未实现此 API
  console.warn('updateStockPosition is not supported in web mode yet');
  return 'success';
};
