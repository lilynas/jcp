package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/run-bigpig/jcp/internal/adk"
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
	configService     *services.ConfigService
	marketService     *services.MarketService
	newsService       *services.NewsService
	hotTrendService   *hottrend.HotTrendService
	meetingService    *meeting.Service
	sessionService    *services.SessionService
	strategyService   *services.StrategyService
	agentContainer    *agent.Container
	toolRegistry      *tools.Registry
	mcpManager        *mcp.Manager
	memoryManager     *memory.Manager
	longHuBangService *services.LongHuBangService

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

	// 初始化龙虎榜服务
	longHuBangService := services.NewLongHuBangService()

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(marketService, newsService, configService, researchReportService, hotTrendSvc, longHuBangService)

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

	// 初始化策略服务和容器
	strategyService := services.NewStrategyService(dataDir)
	agentContainer := agent.NewContainer()
	agentContainer.LoadAgents(strategyService.GetAllAgents())

	log.Println("All services initialized")

	return &Server{
		configService:     configService,
		marketService:     marketService,
		newsService:       newsService,
		hotTrendService:   hotTrendSvc,
		meetingService:    meetingService,
		sessionService:    sessionService,
		strategyService:   strategyService,
		agentContainer:    agentContainer,
		toolRegistry:      toolRegistry,
		mcpManager:        mcpManager,
		memoryManager:     memoryManager,
		longHuBangService: longHuBangService,
		authConfig:        authConfig,
		sessionStore:      NewSessionStore(),
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
	mux.HandleFunc("/api/stock/position", s.authMiddleware(s.handleStockPosition))
	mux.HandleFunc("/api/market/status", s.authMiddleware(s.handleMarketStatus))
	mux.HandleFunc("/api/market/indices", s.authMiddleware(s.handleMarketIndices))
	mux.HandleFunc("/api/agents", s.authMiddleware(s.handleAgents))
	mux.HandleFunc("/api/session", s.authMiddleware(s.handleSession))
	mux.HandleFunc("/api/session/messages", s.authMiddleware(s.handleSessionMessages))
	mux.HandleFunc("/api/meeting/send", s.authMiddleware(s.handleMeetingSend))
	mux.HandleFunc("/api/news/telegraph", s.authMiddleware(s.handleTelegraph))
	mux.HandleFunc("/api/hottrend", s.authMiddleware(s.handleHotTrend))
	mux.HandleFunc("/api/hottrend/platforms", s.authMiddleware(s.handleHotTrendPlatforms))
	mux.HandleFunc("/api/tools", s.authMiddleware(s.handleTools))
	mux.HandleFunc("/api/ai/test", s.authMiddleware(s.handleAITest))
	mux.HandleFunc("/api/mcp/servers", s.authMiddleware(s.handleMCPServers))
	mux.HandleFunc("/api/mcp/status", s.authMiddleware(s.handleMCPStatus))
	mux.HandleFunc("/api/longhubang/list", s.authMiddleware(s.handleLongHuBangList))
	mux.HandleFunc("/api/longhubang/detail", s.authMiddleware(s.handleLongHuBangDetail))

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
		respondJSON(w, s.strategyService.GetAllAgents())
	case "POST":
		var agent models.StrategyAgent
		if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.strategyService.AddAgentToActiveStrategy(agent); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.strategyService.GetAllAgents())
		respondJSON(w, map[string]string{"status": "success"})
	case "PUT":
		var agent models.StrategyAgent
		if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.strategyService.UpdateAgentInActiveStrategy(agent); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.strategyService.GetAllAgents())
		respondJSON(w, map[string]string{"status": "success"})
	case "DELETE":
		id := r.URL.Query().Get("id")
		if id == "" {
			respondError(w, http.StatusBadRequest, "id required")
			return
		}
		if err := s.strategyService.DeleteAgentFromActiveStrategy(id); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.agentContainer.LoadAgents(s.strategyService.GetAllAgents())
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

func (s *Server) handleAITest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var config models.AIConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	factory := adk.NewModelFactory()
	if err := factory.TestConnection(context.Background(), &config); err != nil {
		log.Printf("AI 连接测试失败 [%s]: %v", config.Name, err)
		respondJSON(w, err.Error())
		return
	}

	log.Printf("AI 连接测试成功 [%s]", config.Name)
	respondJSON(w, "success")
}

func (s *Server) handleLongHuBangList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 30
	}
	pageNumber, _ := strconv.Atoi(r.URL.Query().Get("pageNumber"))
	if pageNumber <= 0 {
		pageNumber = 1
	}
	tradeDate := r.URL.Query().Get("tradeDate")

	result, err := s.longHuBangService.GetLongHuBangList(pageSize, pageNumber, tradeDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, result)
}

func (s *Server) handleLongHuBangDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	code := r.URL.Query().Get("code")
	tradeDate := r.URL.Query().Get("tradeDate")

	if code == "" {
		respondError(w, http.StatusBadRequest, "code parameter is required")
		return
	}

	details, err := s.longHuBangService.GetStockDetail(code, tradeDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, details)
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

// ========== Market Data API ==========

func (s *Server) handleMarketStatus(w http.ResponseWriter, r *http.Request) {
	status := s.marketService.GetMarketStatus()
	respondJSON(w, status)
}

func (s *Server) handleMarketIndices(w http.ResponseWriter, r *http.Request) {
	indices, err := s.marketService.GetMarketIndices()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, indices)
}

// ========== Stock Position API ==========

func (s *Server) handleStockPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		StockCode string  `json:"stockCode"`
		Shares    int64   `json:"shares"`
		CostPrice float64 `json:"costPrice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.StockCode == "" {
		respondError(w, http.StatusBadRequest, "stockCode required")
		return
	}
	if err := s.sessionService.UpdatePosition(req.StockCode, req.Shares, req.CostPrice); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

// ========== Meeting API (SSE) ==========

// MeetingMessageRequest 会议室消息请求
type MeetingMessageRequest struct {
	StockCode    string   `json:"stockCode"`
	Content      string   `json:"content"`
	MentionIds   []string `json:"mentionIds"`
	ReplyToId    string   `json:"replyToId"`
	ReplyContent string   `json:"replyContent"`
}

func (s *Server) handleMeetingSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req MeetingMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.StockCode == "" || req.Content == "" {
		respondError(w, http.StatusBadRequest, "stockCode and content required")
		return
	}

	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// 获取Session
	session := s.sessionService.GetSession(req.StockCode)
	if session == nil {
		// 尝试创建
		var err error
		session, err = s.sessionService.GetOrCreateSession(req.StockCode, "")
		if err != nil {
			sendSSEError(w, flusher, "session error: "+err.Error())
			return
		}
	}

	// 保存用户消息
	userMsg := models.ChatMessage{
		AgentID:   "user",
		AgentName: "老韭菜",
		Content:   req.Content,
		ReplyTo:   req.ReplyToId,
		Mentions:  req.MentionIds,
	}
	s.sessionService.AddMessage(req.StockCode, userMsg)

	// 获取股票数据
	stocks, _ := s.marketService.GetStockRealTimeData(req.StockCode)
	var stock models.Stock
	if len(stocks) > 0 {
		stock = stocks[0]
	}

	// 获取默认AI配置
	config := s.configService.GetConfig()
	aiConfig := s.getDefaultAIConfig(config)
	if aiConfig == nil {
		sendSSEError(w, flusher, "未配置 AI 服务，请先在设置中配置")
		return
	}

	// 获取持仓信息
	position := s.sessionService.GetPosition(req.StockCode)

	ctx := r.Context()

	// 响应回调: 每次发言完成后通过 SSE 推送
	respCallback := func(resp meeting.ChatResponse) {
		msg := models.ChatMessage{
			AgentID:   resp.AgentID,
			AgentName: resp.AgentName,
			Role:      resp.Role,
			Content:   resp.Content,
			Round:     resp.Round,
			MsgType:   resp.MsgType,
		}
		s.sessionService.AddMessage(req.StockCode, msg)
		sendSSEEvent(w, flusher, "message", msg)
	}

	// 进度回调: 工具调用等细粒度事件
	progressCallback := func(event meeting.ProgressEvent) {
		sendSSEEvent(w, flusher, "progress", event)
	}

	// 判断模式并执行
	if len(req.MentionIds) == 0 {
		// 智能模式
		allAgents := s.strategyService.GetAllAgents()
		chatReq := meeting.ChatRequest{
			Stock:     stock,
			Query:     req.Content,
			AllAgents: allAgents,
			Position:  position,
		}
		_, err := s.meetingService.RunSmartMeetingWithCallback(ctx, aiConfig, chatReq, respCallback, progressCallback)
		if err != nil {
			sendSSEError(w, flusher, err.Error())
			return
		}
	} else {
		// 直接 @ 指定专家模式
		agentConfigs := s.strategyService.GetAgentsByIDs(req.MentionIds)
		if len(agentConfigs) == 0 {
			sendSSEError(w, flusher, "未找到指定的专家")
			return
		}
		chatReq := meeting.ChatRequest{
			Stock:        stock,
			Agents:       agentConfigs,
			Query:        req.Content,
			ReplyContent: req.ReplyContent,
			Position:     position,
		}
		responses, err := s.meetingService.SendMessage(ctx, aiConfig, chatReq)
		if err != nil {
			sendSSEError(w, flusher, err.Error())
			return
		}
		// 保存并推送响应
		for _, resp := range responses {
			msg := models.ChatMessage{
				AgentID:   resp.AgentID,
				AgentName: resp.AgentName,
				Role:      resp.Role,
				Content:   resp.Content,
				Round:     resp.Round,
				MsgType:   resp.MsgType,
			}
			s.sessionService.AddMessage(req.StockCode, msg)
			sendSSEEvent(w, flusher, "message", msg)
		}
	}

	// 发送完成事件
	sendSSEEvent(w, flusher, "done", map[string]string{"status": "complete"})
}

// getDefaultAIConfig 获取默认AI配置
func (s *Server) getDefaultAIConfig(config *models.AppConfig) *models.AIConfig {
	for i := range config.AIConfigs {
		if config.AIConfigs[i].ID == config.DefaultAIID {
			return &config.AIConfigs[i]
		}
		if config.AIConfigs[i].IsDefault {
			return &config.AIConfigs[i]
		}
	}
	if len(config.AIConfigs) > 0 {
		return &config.AIConfigs[0]
	}
	return nil
}

// sendSSEEvent 发送 SSE 事件
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	flusher.Flush()
}

// sendSSEError 发送 SSE 错误事件
func sendSSEError(w http.ResponseWriter, flusher http.Flusher, message string) {
	sendSSEEvent(w, flusher, "error", map[string]string{"error": message})
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
