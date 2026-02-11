import { isWailsEnv, httpRequest } from './apiAdapter';

// Wails 模式下动态导入
let wailsApi: any = null;
const getWailsApi = async () => {
  if (!wailsApi) {
    wailsApi = await import('@wailsjs/go/main/App');
  }
  return wailsApi;
};

// 策略专属专家配置
export interface StrategyAgent {
  id: string;
  name: string;
  role: string;
  avatar: string;
  color: string;
  instruction: string;
  tools: string[];
  mcpServers: string[];
  enabled: boolean;
  aiConfigId: string;
}

export interface Strategy {
  id: string;
  name: string;
  description: string;
  color: string;
  agents: StrategyAgent[];
  isBuiltin: boolean;
  source: string;
  sourceMeta: string;
  createdAt: number;
}

export interface GenerateStrategyRequest {
  prompt: string;
}

export interface GenerateStrategyResponse {
  success: boolean;
  error?: string;
  strategy?: Strategy;
  reasoning?: string;
}

// 获取所有策略
export const getStrategies = async (): Promise<Strategy[]> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetStrategies();
  }
  return httpRequest<Strategy[]>('/api/strategies');
};

// 获取当前激活策略ID
export const getActiveStrategyID = async (): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GetActiveStrategyID();
  }
  return httpRequest<string>('/api/strategies/active');
};

// 设置当前激活策略
export const setActiveStrategy = async (id: string): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.SetActiveStrategy(id);
  }
  await httpRequest('/api/strategies/active', {
    method: 'PUT',
    body: JSON.stringify({ id }),
  });
  return 'success';
};

// 添加策略
export const addStrategy = async (strategy: Strategy): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.AddStrategy(strategy as any);
  }
  await httpRequest('/api/strategies', {
    method: 'POST',
    body: JSON.stringify(strategy),
  });
  return 'success';
};

// 更新策略
export const updateStrategy = async (strategy: Strategy): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.UpdateStrategy(strategy as any);
  }
  await httpRequest('/api/strategies', {
    method: 'PUT',
    body: JSON.stringify(strategy),
  });
  return 'success';
};

// 删除策略
export const deleteStrategy = async (id: string): Promise<string> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.DeleteStrategy(id);
  }
  await httpRequest(`/api/strategies?id=${id}`, {
    method: 'DELETE',
  });
  return 'success';
};

// AI生成策略
export const generateStrategy = async (prompt: string): Promise<GenerateStrategyResponse> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.GenerateStrategy({ prompt });
  }
  return httpRequest<GenerateStrategyResponse>('/api/strategies/generate', {
    method: 'POST',
    body: JSON.stringify({ prompt }),
  });
};

// 提示词增强请求
export interface EnhancePromptRequest {
  originalPrompt: string;
  agentRole: string;
  agentName: string;
}

// 提示词增强响应
export interface EnhancePromptResponse {
  success: boolean;
  enhancedPrompt?: string;
  error?: string;
}

// 增强Agent提示词
export const enhancePrompt = async (req: EnhancePromptRequest): Promise<EnhancePromptResponse> => {
  if (isWailsEnv()) {
    const api = await getWailsApi();
    return await api.EnhancePrompt(req);
  }
  return httpRequest<EnhancePromptResponse>('/api/strategies/enhance-prompt', {
    method: 'POST',
    body: JSON.stringify(req),
  });
};

// ========== Agent Config API ==========

export interface AgentConfig {
  id: string;
  name: string;
  role: string;
  avatar: string;
  color: string;
  instruction: string;
  tools: string[];
  mcpServers: string[];
  enabled: boolean;
  aiConfigId: string;
}

// 获取所有已启用的Agent配置
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
