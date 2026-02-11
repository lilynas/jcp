package models

// AgentConfig Agent配置（从策略转换而来）
type AgentConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Avatar      string   `json:"avatar"`
	Color       string   `json:"color"`
	Instruction string   `json:"instruction"`
	Tools       []string `json:"tools"`
	MCPServers  []string `json:"mcpServers"`
	Enabled     bool     `json:"enabled"`
	AIConfigID  string   `json:"aiConfigId"` // 可选，空则用默认AI
}
