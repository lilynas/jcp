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

// SSE 回调选项（Web 模式）
export interface MeetingSSECallbacks {
  onMessage?: (msg: ChatMessage) => void;
  onProgress?: (event: any) => void;
  onError?: (error: string) => void;
  onDone?: () => void;
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
// Wails 模式: 调用 Go 绑定，通过事件接收实时响应
// Web 模式: 通过 SSE 接收实时响应
export const sendMeetingMessage = async (req: MeetingMessageRequest, callbacks?: MeetingSSECallbacks): Promise<ChatMessage[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.SendMeetingMessage(req);
  }

  // Web 模式: 使用 SSE (Server-Sent Events) 接收实时流式响应
  return new Promise<ChatMessage[]>((resolve, reject) => {
    const messages: ChatMessage[] = [];
    
    fetch('/api/meeting/send', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }).then(response => {
      if (!response.ok) {
        response.json().then(data => {
          reject(new Error(data.error || 'Request failed'));
        }).catch(() => {
          reject(new Error(`HTTP ${response.status}`));
        });
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) {
        reject(new Error('Streaming not supported'));
        return;
      }

      const decoder = new TextDecoder();
      let buffer = '';

      const processStream = async () => {
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            
            // 解析 SSE 格式: "event: <type>\ndata: <json>\n\n"
            const lines = buffer.split('\n');
            let i = 0;
            while (i < lines.length) {
              // 寻找 event 行
              if (lines[i].startsWith('event: ')) {
                const eventType = lines[i].substring(7).trim();
                // 下一行应该是 data
                if (i + 1 < lines.length && lines[i + 1].startsWith('data: ')) {
                  const dataStr = lines[i + 1].substring(6).trim();
                  // 确保有结束的空行 (说明这条消息完整)
                  if (i + 2 < lines.length && lines[i + 2].trim() === '') {
                    try {
                      const data = JSON.parse(dataStr);
                      
                      switch (eventType) {
                        case 'message': {
                          const msg: ChatMessage = {
                            ...data,
                            id: `msg-${Date.now()}-${Math.random()}`,
                            timestamp: Date.now(),
                          };
                          messages.push(msg);
                          callbacks?.onMessage?.(msg);
                          break;
                        }
                        case 'progress':
                          callbacks?.onProgress?.(data);
                          break;
                        case 'error':
                          callbacks?.onError?.(data.error || 'Unknown error');
                          break;
                        case 'done':
                          callbacks?.onDone?.();
                          break;
                      }
                    } catch (parseErr) {
                      console.warn('[SSE] Parse error:', parseErr, dataStr);
                    }
                    i += 3; // 跳过 event、data、空行
                    continue;
                  }
                }
              }
              i++;
            }
            
            // 保留未处理完的内容（可能是不完整的事件）
            // 找到最后一个完整事件的结束位置
            const lastDoubleNewline = buffer.lastIndexOf('\n\n');
            if (lastDoubleNewline >= 0) {
              buffer = buffer.substring(lastDoubleNewline + 2);
            }
          }
          resolve(messages);
        } catch (err) {
          reject(err);
        }
      };

      processStream();
    }).catch(reject);
  });
};

// 更新股票持仓信息
export const updateStockPosition = async (stockCode: string, shares: number, costPrice: number): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.UpdateStockPosition(stockCode, shares, costPrice);
  }
  await httpRequest('/api/stock/position', {
    method: 'POST',
    body: JSON.stringify({ stockCode, shares, costPrice }),
  });
  return 'success';
};
