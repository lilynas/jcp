import { isWailsEnv, httpRequest } from './apiAdapter';
import { models } from '../../wailsjs/go/models';

export type MCPServerConfig = models.MCPServerConfig;

// MCP 服务器状态
export interface MCPServerStatus {
  id: string;
  connected: boolean;
  error: string;
}

// MCP 工具信息
export interface MCPToolInfo {
  name: string;
  description: string;
  serverId: string;
  serverName: string;
}

// 动态加载 Wails API
let wailsApp: any = null;
const getWailsApp = async () => {
  if (!wailsApp && isWailsEnv()) {
    wailsApp = await import('../../wailsjs/go/main/App');
  }
  return wailsApp;
};

export async function getMCPServers(): Promise<MCPServerConfig[]> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.GetMCPServers();
  } else {
    return httpRequest<MCPServerConfig[]>('/api/mcp/servers');
  }
}

export async function addMCPServer(server: MCPServerConfig): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.AddMCPServer(server);
  } else {
    return httpRequest<string>('/api/mcp/servers', {
      method: 'POST',
      body: JSON.stringify(server),
    });
  }
}

export async function updateMCPServer(server: MCPServerConfig): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.UpdateMCPServer(server);
  } else {
    return httpRequest<string>(`/api/mcp/servers/${server.id}`, {
      method: 'PUT',
      body: JSON.stringify(server),
    });
  }
}

export async function deleteMCPServer(id: string): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.DeleteMCPServer(id);
  } else {
    return httpRequest<string>(`/api/mcp/servers/${id}`, {
      method: 'DELETE',
    });
  }
}

// 获取所有 MCP 服务器状态
export async function getMCPStatus(): Promise<MCPServerStatus[]> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.GetMCPStatus();
  } else {
    return httpRequest<MCPServerStatus[]>('/api/mcp/status');
  }
}

// 测试指定 MCP 服务器连接
export async function testMCPConnection(serverID: string): Promise<MCPServerStatus> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.TestMCPConnection(serverID);
  } else {
    return httpRequest<MCPServerStatus>(`/api/mcp/servers/${serverID}/test`, {
      method: 'POST',
    });
  }
}

// 获取指定 MCP 服务器的工具列表
export async function getMCPServerTools(serverID: string): Promise<MCPToolInfo[]> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.GetMCPServerTools(serverID);
  } else {
    return httpRequest<MCPToolInfo[]>(`/api/mcp/servers/${serverID}/tools`);
  }
}
