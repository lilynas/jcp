package models

// StrategyAgent 策略专属专家配置
type StrategyAgent struct {
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

// Strategy 策略配置
type Strategy struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Color       string          `json:"color"`
	Agents      []StrategyAgent `json:"agents"` // 策略专属的专家配置

	IsBuiltin  bool   `json:"isBuiltin"`
	Source     string `json:"source"`     // builtin/user/ai
	SourceMeta string `json:"sourceMeta"` // AI生成时的原始prompt
	CreatedAt  int64  `json:"createdAt"`
}

// StrategyStore 策略存储结构
type StrategyStore struct {
	ActiveID   string     `json:"activeId"`
	Strategies []Strategy `json:"strategies"`
}
