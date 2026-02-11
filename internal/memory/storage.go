package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Storage 存储接口
type Storage interface {
	Load(stockCode string) (*StockMemory, error)
	Save(mem *StockMemory) error
	Delete(stockCode string) error
	List() ([]string, error)
}

// FileStorage 文件存储（按股票隔离）
type FileStorage struct {
	dir   string
	cache map[string]*StockMemory
	mu    sync.RWMutex
}

// NewFileStorage 创建文件存储
func NewFileStorage(dataDir string) *FileStorage {
	memDir := filepath.Join(dataDir, "memories")
	os.MkdirAll(memDir, 0755)
	return &FileStorage{
		dir:   memDir,
		cache: make(map[string]*StockMemory),
	}
}

// getPath 获取存储路径
func (s *FileStorage) getPath(stockCode string) string {
	return filepath.Join(s.dir, stockCode+".json")
}

// Load 加载股票记忆
func (s *FileStorage) Load(stockCode string) (*StockMemory, error) {
	s.mu.RLock()
	if mem, ok := s.cache[stockCode]; ok {
		s.mu.RUnlock()
		return mem, nil
	}
	s.mu.RUnlock()

	// 从文件加载
	data, err := os.ReadFile(s.getPath(stockCode))
	if err != nil {
		return nil, err
	}

	var mem StockMemory
	if err := json.Unmarshal(data, &mem); err != nil {
		return nil, err
	}

	// 缓存
	s.mu.Lock()
	s.cache[stockCode] = &mem
	s.mu.Unlock()

	return &mem, nil
}

// Save 保存股票记忆
func (s *FileStorage) Save(mem *StockMemory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(s.getPath(mem.StockCode), data, 0644); err != nil {
		return err
	}

	s.cache[mem.StockCode] = mem
	return nil
}

// Delete 删除股票记忆
func (s *FileStorage) Delete(stockCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cache, stockCode)
	err := os.Remove(s.getPath(stockCode))
	if err != nil && os.IsNotExist(err) {
		return nil // 文件不存在，无需删除
	}
	return err
}

// List 列出所有股票记忆
func (s *FileStorage) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			code := e.Name()[:len(e.Name())-5]
			codes = append(codes, code)
		}
	}
	return codes, nil
}

// Invalidate 清除缓存
func (s *FileStorage) Invalidate(stockCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, stockCode)
}
