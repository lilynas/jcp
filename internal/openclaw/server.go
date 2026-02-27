package openclaw

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/run-bigpig/jcp/internal/agent"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/models"
)

var log = logger.New("OpenClaw")

// StockResolver 根据股票代码获取实时数据
type StockResolver func(code string) (*models.Stock, error)

// Server OpenClaw HTTP 服务
type Server struct {
	mu             sync.RWMutex
	server         *http.Server
	port           int
	apiKey         string
	meetingService *meeting.Service
	agentContainer *agent.Container
	aiResolver     func(string) *models.AIConfig
	stockResolver  StockResolver
}

// NewServer 创建 OpenClaw 服务
func NewServer(ms *meeting.Service, ac *agent.Container, resolver func(string) *models.AIConfig, stockResolver StockResolver) *Server {
	return &Server{
		meetingService: ms,
		agentContainer: ac,
		aiResolver:     resolver,
		stockResolver:  stockResolver,
	}
}

// Start 启动服务
func (s *Server) Start(port int, apiKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return fmt.Errorf("服务已在运行")
	}

	// 先检测端口是否可用
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("端口 %d 被占用", port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/analyze", s.withAuth(s.handleAnalyze))

	s.port = port
	s.apiKey = apiKey
	s.server = &http.Server{Handler: mux}

	go func() {
		log.Info("OpenClaw 服务启动于端口 %d", port)
		if err := s.server.Serve(ln); err != http.ErrServerClosed {
			log.Error("服务异常: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()

	err := s.server.Shutdown(ctx)
	s.server = nil
	log.Info("OpenClaw 服务已停止")
	return err
}

// Restart 重启服务（端口或密钥变更时调用）
func (s *Server) Restart(port int, apiKey string) error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start(port, apiKey)
}

// IsRunning 检查服务是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.server != nil
}

// GetPort 获取当前端口
func (s *Server) GetPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}
