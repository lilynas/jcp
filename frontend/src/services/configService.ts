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
