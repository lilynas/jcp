// 配置服务 - 支持 Wails 和 Web 双模式
import { isWailsEnv, httpRequest } from './apiAdapter';

// 内置工具信息
export interface ToolInfo {
  name: string;
  description: string;
}

// AppConfig 类型定义 (与后端 models.AppConfig 对应)
export interface AppConfig {
  aiConfigs?: any[];
  defaultAIID?: string;
  mcpServers?: any[];
  memory?: any;
  theme?: string;
  layout?: {
    leftPanelWidth: number;
    rightPanelWidth: number;
    bottomPanelHeight: number;
    windowWidth: number;
    windowHeight: number;
  };
  [key: string]: any; // 允许其他属性
}

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

export const getConfig = async (): Promise<AppConfig> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetConfig();
  }
  return httpRequest<AppConfig>('/api/config');
};

export const updateConfig = async (config: AppConfig): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.UpdateConfig(config);
  }
  await httpRequest('/api/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
  return 'success';
};

// 获取可用的内置工具列表
export const getAvailableTools = async (): Promise<ToolInfo[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetAvailableTools();
  }
  return httpRequest<ToolInfo[]>('/api/tools');
};

// 测试 AI 配置连通性
export const testAIConnection = async (config: any): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.TestAIConnection(config);
  }
  return httpRequest<string>('/api/ai/test', {
    method: 'POST',
    body: JSON.stringify(config),
  });
};

// 龙虎榜数据类型
export interface LongHuBangItem {
  tradeDate: string;
  code: string;
  secuCode?: string;
  name: string;
  closePrice: number;
  changePercent: number;
  netBuyAmt: number;
  buyAmt: number;
  sellAmt: number;
  totalAmt?: number;
  turnoverRate: number;
  freeCap?: number;
  reason: string;
  reasonDetail?: string;
  accumAmount?: number;
  dealRatio: number;
  netRatio?: number;
  d1Change: number;
  d2Change?: number;
  d5Change: number;
  d10Change: number;
  securityType?: string;
}

export interface LongHuBangDetail {
  rank: number;
  operName: string;
  buyAmt: number;
  buyPercent: number;
  sellAmt: number;
  sellPercent: number;
  netAmt?: number;
  direction: string;
}

export interface LongHuBangListResult {
  items: LongHuBangItem[];
  total: number;
}

// 获取龙虎榜列表
export const getLongHuBangList = async (pageSize: number, pageNumber: number, tradeDate: string): Promise<LongHuBangListResult> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetLongHuBangList(pageSize, pageNumber, tradeDate);
  }
  const params = new URLSearchParams({ pageSize: String(pageSize), pageNumber: String(pageNumber), tradeDate });
  return httpRequest<LongHuBangListResult>(`/api/longhubang/list?${params}`);
};

// 获取龙虎榜详情
export const getLongHuBangDetail = async (code: string, tradeDate: string): Promise<LongHuBangDetail[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetLongHuBangDetail(code, tradeDate);
  }
  const params = new URLSearchParams({ code, tradeDate });
  return httpRequest<LongHuBangDetail[]>(`/api/longhubang/detail?${params}`);
};
