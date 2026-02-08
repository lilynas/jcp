import { isWailsEnv, httpRequest } from './apiAdapter';

export interface AgentConfig {
  id: string;
  name: string;
  role: string;
  avatar: string;
  color: string;
  instruction: string;
  tools: string[];
  mcpServers: string[];  // 关联的 MCP 服务器 ID 列表
  priority: number;
  isBuiltin: boolean;
  enabled: boolean;
  providerId: string;  // 关联的Provider ID（空则使用默认）
}

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

// 获取所有Agent配置
export const getAgentConfigs = async (): Promise<AgentConfig[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetAgentConfigs();
  }
  return httpRequest<AgentConfig[]>('/api/agents');
};

// 添加Agent配置
export const addAgentConfig = async (config: AgentConfig): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.AddAgentConfig(config);
  }
  await httpRequest('/api/agents', {
    method: 'POST',
    body: JSON.stringify(config),
  });
  return 'success';
};

// 更新Agent配置
export const updateAgentConfig = async (config: AgentConfig): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.UpdateAgentConfig(config);
  }
  await httpRequest('/api/agents', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
  return 'success';
};

// 删除Agent配置
export const deleteAgentConfig = async (id: string): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.DeleteAgentConfig(id);
  }
  await httpRequest(`/api/agents?id=${id}`, {
    method: 'DELETE',
  });
  return 'success';
};
