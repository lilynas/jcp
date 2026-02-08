package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/agent"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/memory"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"
)

//go:embed all:static
var staticFiles embed.FS

// Version 版本号，通过 ldflags 注入
var Version = "dev"

// ========== 认证相关 ==========

// AuthConfig 认证配置
type AuthConfig struct {
	Password    string        // 登录密码 (从环境变量 JCP_PASSWORD 读取)
	TokenExpiry time.Duration // Token 过期时间
	AuthEnabled bool          // 是否启用认证
}

// SessionStore 会话存储
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token -> 过期时间
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]time.Time),
	}
}

func (s *SessionStore) Create(expiry time.Duration) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(expiry)
	s.mu.Unlock()
	return token
}

func (s *SessionStore) Validate(token string) bool {
	s.mu.RLock()
	expiry, exists := s.sessions[token]
	s.mu.RUnlock()
	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		s.Delete(token)
		return false
	}
	return true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *SessionStore) Refresh(token string, expiry time.Duration) {
	s.mu.Lock()
	if _, exists := s.sessions[token]; exists {
		s.sessions[token] = time.Now().Add(expiry)
	}
	s.mu.Unlock()
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Server 封装 Web 服务需要的服务
type Server struct {
	configService      *services.ConfigService
	marketService      *services.MarketService
	newsService        *services.NewsService
	hotTrendService    *hottrend.HotTrendService
	meetingService     *meeting.Service
	sessionService     *services.SessionService
	agentConfigService *services.AgentConfigService
	agentContainer     *agent.Container
	toolRegistry       *tools.Registry
	mcpManager         *mcp.Manager
	memoryManager      *memory.Manager

	// 认证相关
	authConfig   AuthConfig
	sessionStore *SessionStore
}

func main() {
	log.Printf("JCP Web Server %s starting...", Version)

	// 获取数据目录
	dataDir := getDataDir()

	// 初始化日志
	if err := logger.InitFileLogger(filepath.Join(dataDir, "logs")); err != nil {
		log.Printf("Init file logger error: %v", err)
	}
	logger.SetGlobalLevel(logger.DEBUG)

	// 认证配置
	authConfig := AuthConfig{
		Password:    os.Getenv("JCP_PASSWORD"),
		TokenExpiry: 24 * time.Hour,
		AuthEnabled: os.Getenv("JCP_PASSWORD") != "",
	}

	if authConfig.AuthEnabled {
		log.Println("Authentication ENABLED (JCP_PASSWORD is set)")
	} else {
		log.Println("Authentication DISABLED (set JCP_PASSWORD to enable)")
	}

	// 初始化服务
	srv, err := newServer(dataDir, authConfig)
	if err != nil {
		log.Fatalf("Failed to init server: %v", err)
	}

	// 注册路由
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// 启动 HTTP 服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	// 优雅关闭
	go func() {
		log.Printf("Server listening on http://0.0.0.0:%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	logger.Close()
	log.Println("Server stopped")
}

func getDataDir() string {
	// Docker 环境优先使用 /app/data
	if _, err := os.Stat("/app/data"); err == nil {
		return "/app/data"
	}
	// 环境变量
	if dir := os.Getenv("JCP_DATA_DIR"); dir != "" {
		return dir
	}
	// 本地开发使用 ./data
	return "./data"
}

func newServer(dataDir string, authConfig AuthConfig) (*Server, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	// 初始化配置服务
	configService, err := services.NewConfigService(dataDir)
	if err != nil {
		return nil, err
	}

	// 初始化各种服务
	researchReportService := services.NewResearchReportService()
	hotTrendSvc, _ := hottrend.NewHotTrendService()
	marketService := services.NewMarketService()
	newsService := services.NewNewsService()

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(marketService, newsService, configService, researchReportService, hotTrendSvc)

	// 初始化 MCP 管理器
	mcpManager := mcp.NewManager()
	if err := mcpManager.LoadConfigs(configService.GetConfig().MCPServers); err != nil {
		log.Printf("MCP load error: %v", err)
	}

	// 初始化会议室服务
	meetingService := meeting.NewServiceFull(toolRegistry, mcpManager)

	// 初始化记忆管理器
	var memoryManager *memory.Manager
	memConfig := configService.GetConfig().Memory
	if memConfig.Enabled {
		memoryManager = memory.NewManagerWithConfig(dataDir, memory.Config{
			MaxRecentRounds:   memConfig.MaxRecentRounds,
			MaxKeyFacts:       memConfig.MaxKeyFacts,
			MaxSummaryLength:  memConfig.MaxSummaryLength,
			CompressThreshold: memConfig.CompressThreshold,
		})
		meetingService.SetMemoryManager(memoryManager)

		if memConfig.AIConfigID != "" {
			for i := range configService.GetConfig().AIConfigs {
				if configService.GetConfig().AIConfigs[i].ID == memConfig.AIConfigID {
					meetingService.SetMemoryAIConfig(&configService.GetConfig().AIConfigs[i])
					break
				}
			}
		}
	}

	// 初始化Session服务
	sessionService := services.NewSessionService(dataDir)

	// 初始化Agent配置服务和容器
	agentConfigService := services.NewAgentConfigService(dataDir)
	agentContainer := agent.NewContainer()
	agentContainer.LoadAgents(agentConfigService.GetAllAgents())

	log.Println("All services initialized")

	return &Server{
		configService:      configService,
		marketService:      marketService,
		newsService:        newsService,
		hotTrendService:    hotTrendSvc,
		meetingService:     meetingService,
		sessionService:     sessionService,
		agentConfigService: agentConfigService,
		agentContainer:     agentContainer,
		toolRegistry:       toolRegistry,
		mcpManager:         mcpManager,
		memoryManager:      memoryManager,
		authConfig:         authConfig,
		sessionStore:       NewSessionStore(),
	}, nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 公开路由 (无需认证)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)

	// 受保护的 API 路由 (需要认证)
	mux.HandleFunc("/api/config", s.authMiddleware(s.handleConfig))
	mux.HandleFunc("/api/watchlist", s.authMiddleware(s.handleWatchlist))
	mux.HandleFunc("/api/stock/realtime", s.authMiddleware(s.handleStockRealtime))
	mux.HandleFunc("/api/stock/kline", s.authMiddleware(s.handleKLine))
	mux.HandleFunc("/api/stock/orderbook", s.authMiddleware(s.handleOrderBook))
	mux.HandleFunc("/api/stock/search", s.authMiddleware(s.handleSearchStocks))
	mux.HandleFunc("/api/agents", s.authMiddleware(s.handleAgents))
	mux.HandleFunc("/api/session", s.authMiddleware(s.handleSession))
	mux.HandleFunc("/api/session/messages", s.authMiddleware(s.handleSessionMessages))
	mux.HandleFunc("/api/news/telegraph", s.authMiddleware(s.handleTelegraph))
	mux.HandleFunc("/api/hottrend", s.authMiddleware(s.handleHotTrend))
	mux.HandleFunc("/api/hottrend/platforms", s.authMiddleware(s.handleHotTrendPlatforms))
	mux.HandleFunc("/api/tools", s.authMiddleware(s.handleTools))
	mux.HandleFunc("/api/mcp/servers", s.authMiddleware(s.handleMCPServers))
	mux.HandleFunc("/api/mcp/status", s.authMiddleware(s.handleMCPStatus))

	// 静态文件服务 (前端)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("Static files not embedded, serving from ./frontend/dist")
		mux.Handle("/", http.FileServer(http.Dir("./frontend/dist")))
	} else {
		mux.Handle("/", spaHandler(http.FileServer(http.FS(staticFS))))
	}
}

// ========== 认证中间件 ==========

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果未启用认证，直接放行
		if !s.authConfig.AuthEnabled {
			next(w, r)
			return
		}

		// 从 Cookie 或 Header 获取 token
		token := ""
		if cookie, err := r.Cookie("jcp_token"); err == nil {
			token = cookie.Value
		}
		if token == "" {
			token = r.Header.Get("Authorization")
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
			}
		}

		// 验证 token
		if token == "" || !s.sessionStore.Validate(token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		// 刷新 token 过期时间
		s.sessionStore.Refresh(token, s.authConfig.TokenExpiry)

		next(w, r)
	}
}

// ========== 认证 API ==========

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 如果未启用认证，直接返回成功
	if !s.authConfig.AuthEnabled {
		respondJSON(w, map[string]interface{}{
			"success":      true,
			"authRequired": false,
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// 验证密码
	if req.Password != s.authConfig.Password {
		respondError(w, http.StatusUnauthorized, "密码错误")
		return
	}

	// 创建会话
	token := s.sessionStore.Create(s.authConfig.TokenExpiry)

	// 设置 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jcp_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(s.authConfig.TokenExpiry.Seconds()),
		SameSite: http.SameSiteStrictMode,
	})

	respondJSON(w, map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从 Cookie 获取 token 并删除
	if cookie, err := r.Cookie("jcp_token"); err == nil {
		s.sessionStore.Delete(cookie.Value)
	}

	// 清除 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jcp_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	respondJSON(w, map[string]string{"status": "success"})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	// 检查是否需要认证
	authRequired := s.authConfig.AuthEnabled
	authenticated := false

	if authRequired {
		// 检查当前是否已登录
		token := ""
		if cookie, err := r.Cookie("jcp_token"); err == nil {
			token = cookie.Value
		}
		if token == "" {
			token = r.Header.Get("Authorization")
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
			}
		}
		authenticated = token != "" && s.sessionStore.Validate(token)
	} else {
		authenticated = true
	}

	respondJSON(w, map[string]interface{}{
		"authRequired":  authRequired,
		"authenticated": authenticated,
	})
}

// SPA 处理器：所有非 API 请求返回 index.html
func spaHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API 请求直接返回 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// CORS 中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ========== API Handlers ==========

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"version": Version})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		respondJSON(w, s.configService.GetConfig())
	case "PUT", "POST":
		var config models.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.configService.UpdateConfig(&config); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 重新加载 MCP 配置
		if s.mcpManager != nil && config.MCPServers != nil {
			s.mcpManager.LoadConfigs(config.MCPServers)
		}
		respondJSON(w, map[string]string{"status": "success"})
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleWatchlist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		respondJSON(w, s.configService.GetWatchlist())
	case "POST":
		var stock models.Stock
		if err := json.NewDecoder(r.Body).Decode(&stock); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.configService.AddToWatchlist(stock); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, map[string]string{"status": "success"})
	case "DELETE":
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			respondError(w, http.StatusBadRequest, "symbol required")
			return
		}
		if err := s.configService.RemoveFromWatchlist(symbol); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, map[string]string{"status": "success"})
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleStockRealtime(w http.ResponseWriter, r *http.Request) {
	codes := r.URL.Query().Get("codes")
	if codes == "" {
		respondError(w, http.StatusBadRequest, "codes required")
		return
	}
	codeList := strings.Split(codes, ",")
	stocks, err := s.marketService.GetStockRealTimeData(codeList...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, stocks)
}

func (s *Server) handleKLine(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	period := r.URL.Query().Get("period")
	if code == "" {
		respondError(w, http.StatusBadRequest, "code required")
		return
	}
	if period == "" {
		period = "day"
	}
	data, err := s.marketService.GetKLineData(code, period, 120)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, data)
}

func (s *Server) handleOrderBook(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, http.StatusBadRequest, "code required")
		return
	}
	orderBook, err := s.marketService.GetRealOrderBook(code)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, orderBook)
}

func (s *Server) handleSearchStocks(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		respondJSON(w, []services.StockSearchResult{})
		return
	}
	results := s.configService.SearchStocks(keyword, 20)
	respondJSON(w, results)
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		respondJSON(w, s.agentConfigService.GetAllAgents())
	case "POST":
		var config models.AgentConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.agentConfigService.AddAgent(config); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.agentConfigService.GetAllAgents())
		respondJSON(w, map[string]string{"status": "success"})
	case "PUT":
		var config models.AgentConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.agentConfigService.UpdateAgent(config); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.agentConfigService.GetAllAgents())
		respondJSON(w, map[string]string{"status": "success"})
	case "DELETE":
		id := r.URL.Query().Get("id")
		if id == "" {
			respondError(w, http.StatusBadRequest, "id required")
			return
		}
		if err := s.agentConfigService.DeleteAgent(id); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.agentConfigService.GetAllAgents())
		respondJSON(w, map[string]string{"status": "success"})
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	stockCode := r.URL.Query().Get("stockCode")
	stockName := r.URL.Query().Get("stockName")
	if stockCode == "" {
		respondError(w, http.StatusBadRequest, "stockCode required")
		return
	}
	session, err := s.sessionService.GetOrCreateSession(stockCode, stockName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, session)
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	stockCode := r.URL.Query().Get("stockCode")
	if stockCode == "" {
		respondError(w, http.StatusBadRequest, "stockCode required")
		return
	}

	switch r.Method {
	case "GET":
		messages := s.sessionService.GetMessages(stockCode)
		respondJSON(w, messages)
	case "DELETE":
		if err := s.sessionService.ClearMessages(stockCode); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.memoryManager != nil {
			s.memoryManager.DeleteMemory(stockCode)
		}
		respondJSON(w, map[string]string{"status": "success"})
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleTelegraph(w http.ResponseWriter, r *http.Request) {
	telegraphs, err := s.newsService.GetTelegraphList()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, telegraphs)
}

func (s *Server) handleHotTrendPlatforms(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, hottrend.SupportedPlatforms)
}

func (s *Server) handleHotTrend(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		// 返回所有平台
		if s.hotTrendService == nil {
			respondJSON(w, []hottrend.HotTrendResult{})
			return
		}
		respondJSON(w, s.hotTrendService.GetAllHotTrends())
		return
	}
	// 返回单个平台
	if s.hotTrendService == nil {
		respondJSON(w, hottrend.HotTrendResult{Platform: platform, Error: "服务未初始化"})
		return
	}
	respondJSON(w, s.hotTrendService.GetHotTrend(platform))
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, s.toolRegistry.GetAllToolInfos())
}

func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	config := s.configService.GetConfig()
	if config.MCPServers == nil {
		respondJSON(w, []models.MCPServerConfig{})
		return
	}
	respondJSON(w, config.MCPServers)
}

func (s *Server) handleMCPStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, s.mcpManager.GetAllStatus())
}

// ========== Helper functions ==========

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
