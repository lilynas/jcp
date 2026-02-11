// Package mcp 提供 MCP (Model Context Protocol) 集成功能
// 采用 adk-go 官方设计，直接使用 mcptoolset
package mcp

import (
	"context"
	"os/exec"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

var log = logger.New("mcp")

// ServerStatus MCP 服务器状态
type ServerStatus struct {
	ID        string `json:"id"`
	Connected bool   `json:"connected"`
	Error     string `json:"error"`
}

// ToolInfo MCP 工具信息
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ServerID    string `json:"serverId"`
	ServerName  string `json:"serverName"`
}

// Manager MCP 服务管理器
// 负责配置管理和缓存 mcptoolset，生命周期绑定主 context
type Manager struct {
	ctx      context.Context
	mu       sync.RWMutex
	configs  map[string]*models.MCPServerConfig
	toolsets map[string]tool.Toolset // 缓存已创建的 toolset
}

// NewManager 创建 MCP 管理器（需要调用 Initialize 绑定 context）
func NewManager() *Manager {
	return &Manager{
		configs:  make(map[string]*models.MCPServerConfig),
		toolsets: make(map[string]tool.Toolset),
	}
}

// Initialize 初始化管理器，绑定主 context 并预创建所有已配置的 toolset
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx = ctx

	// 预初始化所有已配置的 toolset
	for id, cfg := range m.configs {
		if _, ok := m.toolsets[id]; ok {
			continue
		}
		ts, err := m.createToolsetLocked(cfg)
		if err != nil {
			log.Warn("预初始化 toolset 失败 [%s]: %v", cfg.Name, err)
			continue
		}
		m.toolsets[id] = ts
		log.Info("预初始化 toolset 成功: %s", cfg.Name)
	}
	return nil
}

// LoadConfigs 加载 MCP 服务器配置（会清空已缓存的 toolset，并在已初始化时自动创建新 toolset）
func (m *Manager) LoadConfigs(configs []models.MCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空旧配置和缓存
	m.configs = make(map[string]*models.MCPServerConfig)
	m.toolsets = make(map[string]tool.Toolset)

	for i := range configs {
		cfg := &configs[i]
		if !cfg.Enabled {
			continue
		}
		m.configs[cfg.ID] = cfg
		log.Info("加载 MCP 配置: %s (%s)", cfg.Name, cfg.TransportType)
	}

	// 如果已绑定 context，则预初始化所有 toolset
	if m.ctx != nil {
		for id, cfg := range m.configs {
			ts, err := m.createToolsetLocked(cfg)
			if err != nil {
				log.Warn("初始化 toolset 失败 [%s]: %v", cfg.Name, err)
				continue
			}
			m.toolsets[id] = ts
			log.Info("初始化 toolset 成功: %s", cfg.Name)
		}
	}
	return nil
}

// createTransport 根据配置创建 MCP 传输层
func createTransport(cfg *models.MCPServerConfig) mcp.Transport {
	switch cfg.TransportType {
	case models.MCPTransportSSE:
		log.Warn("创建 SSE 传输 [%s]: %s (已废弃)", cfg.Name, cfg.Endpoint)
		return &mcp.SSEClientTransport{Endpoint: cfg.Endpoint}
	case models.MCPTransportCommand:
		log.Info("创建 Command 传输 [%s]: %s %v", cfg.Name, cfg.Command, cfg.Args)
		return &mcp.CommandTransport{Command: exec.Command(cfg.Command, cfg.Args...)}
	default:
		log.Info("创建 StreamableHTTP 传输 [%s]: %s", cfg.Name, cfg.Endpoint)
		return &mcp.StreamableClientTransport{
			Endpoint:   cfg.Endpoint,
			MaxRetries: 3,
		}
	}
}

// CreateToolset 为指定配置创建 mcptoolset（直接使用 adk-go 官方实现）
func (m *Manager) CreateToolset(cfg *models.MCPServerConfig) (tool.Toolset, error) {
	return m.createToolsetLocked(cfg)
}

// createToolsetLocked 内部方法，创建 toolset（调用方需持有锁）
func (m *Manager) createToolsetLocked(cfg *models.MCPServerConfig) (tool.Toolset, error) {
	ts, err := mcptoolset.New(mcptoolset.Config{
		Transport: createTransport(cfg),
	})
	if err != nil {
		log.Error("创建 mcptoolset 失败 [%s]: %v", cfg.Name, err)
		return nil, err
	}
	log.Debug("mcptoolset 已创建: %s", cfg.Name)
	return ts, nil
}

// GetToolsetsByIDs 根据 ID 列表获取 toolsets（使用缓存）
func (m *Manager) GetToolsetsByIDs(ids []string) []tool.Toolset {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Info("请求获取 toolsets, IDs: %v", ids)
	var result []tool.Toolset
	for _, id := range ids {
		// 先检查缓存
		if ts, ok := m.toolsets[id]; ok {
			log.Debug("使用缓存的 toolset: %s", id)
			result = append(result, ts)
			continue
		}

		// 缓存未命中，创建新的
		cfg, ok := m.configs[id]
		if !ok {
			log.Warn("MCP 配置不存在: %s", id)
			continue
		}
		ts, err := m.createToolsetLocked(cfg)
		if err != nil {
			log.Error("创建 toolset 失败 [%s]: %v", id, err)
			continue
		}
		// 存入缓存
		m.toolsets[id] = ts
		result = append(result, ts)
	}
	log.Info("返回 toolsets 数量: %d", len(result))
	return result
}

// GetAllToolsets 获取所有已启用的 toolsets（使用缓存）
func (m *Manager) GetAllToolsets() []tool.Toolset {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []tool.Toolset
	for id, cfg := range m.configs {
		// 先检查缓存
		if ts, ok := m.toolsets[id]; ok {
			result = append(result, ts)
			continue
		}
		// 创建并缓存
		ts, err := m.createToolsetLocked(cfg)
		if err != nil {
			continue
		}
		m.toolsets[id] = ts
		result = append(result, ts)
	}
	return result
}

// GetAllStatus 获取所有服务器状态
func (m *Manager) GetAllStatus() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServerStatus, 0, len(m.configs))
	for id := range m.configs {
		result = append(result, ServerStatus{ID: id})
	}
	return result
}

// TestConnection 测试指定 MCP 服务器的连接
func (m *Manager) TestConnection(serverID string) *ServerStatus {
	log.Info("测试连接: %s", serverID)
	m.mu.RLock()
	cfg, ok := m.configs[serverID]
	m.mu.RUnlock()

	if !ok {
		return &ServerStatus{ID: serverID, Connected: false, Error: "服务器未配置"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	impl := &mcp.Implementation{Name: cfg.Name, Version: "1.0.0"}
	client := mcp.NewClient(impl, nil)
	_, err := client.Connect(ctx, createTransport(cfg), nil)

	if err != nil {
		log.Error("测试连接失败 [%s]: %v", cfg.Name, err)
		return &ServerStatus{ID: serverID, Connected: false, Error: err.Error()}
	}
	log.Info("测试连接成功: %s", cfg.Name)
	return &ServerStatus{ID: serverID, Connected: true}
}

// GetServerTools 获取指定 MCP 服务器的工具列表
func (m *Manager) GetServerTools(serverID string) ([]ToolInfo, error) {
	m.mu.RLock()
	cfg, ok := m.configs[serverID]
	m.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	impl := &mcp.Implementation{Name: cfg.Name, Version: "1.0.0"}
	client := mcp.NewClient(impl, nil)
	session, err := client.Connect(ctx, createTransport(cfg), nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	toolsResp, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}

	var tools []ToolInfo
	for _, t := range toolsResp.Tools {
		tools = append(tools, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			ServerID:    serverID,
			ServerName:  cfg.Name,
		})
	}
	return tools, nil
}

// GetToolInfosByServerIDs 根据服务器 ID 列表获取工具信息
func (m *Manager) GetToolInfosByServerIDs(serverIDs []string) []ToolInfo {
	log.Info("获取工具信息, 服务器IDs: %v", serverIDs)
	var allTools []ToolInfo
	for _, id := range serverIDs {
		tools, err := m.GetServerTools(id)
		if err != nil {
			log.Error("获取服务器工具失败 [%s]: %v", id, err)
			continue
		}
		if tools != nil {
			allTools = append(allTools, tools...)
		}
	}
	log.Info("共获取 %d 个工具", len(allTools))
	return allTools
}
