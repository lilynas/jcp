package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

	// 初始化服务
	srv, err := newServer(dataDir)
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

func newServer(dataDir string) (*Server, error) {
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
	}, nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API 路由
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/watchlist", s.handleWatchlist)
	mux.HandleFunc("/api/stock/realtime", s.handleStockRealtime)
	mux.HandleFunc("/api/stock/kline", s.handleKLine)
	mux.HandleFunc("/api/stock/orderbook", s.handleOrderBook)
	mux.HandleFunc("/api/stock/search", s.handleSearchStocks)
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/session/messages", s.handleSessionMessages)
	mux.HandleFunc("/api/news/telegraph", s.handleTelegraph)
	mux.HandleFunc("/api/hottrend", s.handleHotTrend)
	mux.HandleFunc("/api/hottrend/platforms", s.handleHotTrendPlatforms)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/mcp/servers", s.handleMCPServers)
	mux.HandleFunc("/api/mcp/status", s.handleMCPStatus)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/health", s.handleHealth)

	// 静态文件服务 (前端)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("Static files not embedded, serving from ./frontend/dist")
		mux.Handle("/", http.FileServer(http.Dir("./frontend/dist")))
	} else {
		mux.Handle("/", spaHandler(http.FileServer(http.FS(staticFS))))
	}
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
