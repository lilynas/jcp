package openclaw

import (
	"encoding/json"
	"net/http"
	"strings"
)

// withAuth 鉴权中间件
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey != "" {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.apiKey {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	agents := s.agentContainer.GetAllAgents()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"agentCount":  len(agents),
		"aiConfigured": s.aiResolver("") != nil,
	})
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
