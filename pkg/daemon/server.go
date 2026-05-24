package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// VMConfig holds the specs for a new Unikraft Cloud virtual machine instance.
type VMConfig struct {
	ImageName string            `json:"image_name"`
	Env       map[string]string `json:"env"`
	MemoryMB  int               `json:"memory_mb"`
}

// VMInstance represents the operational and network identity of a multi-tenant VM.
type VMInstance struct {
	ID    string `json:"id"`
	State string `json:"state"`
	IP    string `json:"ip"`
}

// Server implements a lightweight, secure REST API server for the drover-runner host daemon.
type Server struct {
	cfg    Config
	driver HypervisorDriver
	vms    sync.Map // maps id string -> *ActiveVM
	vmSeq  uint64
	mu     sync.Mutex
}

// NewServer returns a new authenticated daemon API REST server.
func NewServer(cfg Config, driver HypervisorDriver) *Server {
	return &Server{
		cfg:    cfg,
		driver: driver,
	}
}

// ServeHTTP implements the http.Handler interface and routes secure VM REST operations.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authentication
	authHeader := r.Header.Get("Authorization")
	expectedAuth := fmt.Sprintf("Bearer %s", s.cfg.AuthToken)
	if s.cfg.AuthToken != "" && authHeader != expectedAuth {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Router
	path := r.URL.Path
	method := r.Method

	if path == "/v1/instances" {
		if method == http.MethodPost {
			s.handleCreateInstance(w, r)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if strings.HasPrefix(path, "/v1/instances/") {
		parts := strings.Split(strings.TrimPrefix(path, "/v1/instances/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			id := parts[0]
			if method == http.MethodGet {
				s.handleGetInstance(w, r, id)
				return
			} else if method == http.MethodDelete {
				s.handleDeleteInstance(w, r, id)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "logs" {
			id := parts[0]
			if method == http.MethodGet {
				s.handleGetInstanceLogs(w, r, id)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req VMConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json body"})
		return
	}

	s.mu.Lock()
	s.vmSeq++
	id := fmt.Sprintf("inst-%d", s.vmSeq)
	s.mu.Unlock()

	active, err := s.driver.LaunchVM(r.Context(), id, req.ImageName, req.MemoryMB, req.Env)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	s.vms.Store(id, active)

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(VMInstance{
		ID:    active.ID,
		State: active.State,
		IP:    active.IP,
	})
}

func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request, id string) {
	val, ok := s.vms.Load(id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "instance not found"})
		return
	}
	active := val.(*ActiveVM)

	_ = json.NewEncoder(w).Encode(VMInstance{
		ID:    active.ID,
		State: active.State,
		IP:    active.IP,
	})
}

func (s *Server) handleDeleteInstance(w http.ResponseWriter, r *http.Request, id string) {
	val, ok := s.vms.Load(id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "instance not found"})
		return
	}
	active := val.(*ActiveVM)

	if err := active.Stop(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to stop instance: %v", err)})
		return
	}

	s.vms.Delete(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetInstanceLogs(w http.ResponseWriter, r *http.Request, id string) {
	val, ok := s.vms.Load(id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "instance not found"})
		return
	}
	active := val.(*ActiveVM)

	logs, err := active.Logs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"logs": logs})
}

// StopAll terminates all active Guest VM instances and cleans up runtime traces.
func (s *Server) StopAll() {
	s.vms.Range(func(key, val interface{}) bool {
		id := key.(string)
		active := val.(*ActiveVM)
		fmt.Printf("   [daemon] Stopping instance %s...\n", id)
		if err := active.Stop(); err != nil {
			fmt.Printf("   [daemon] Error stopping instance %s: %v\n", id, err)
		}
		s.vms.Delete(id)
		return true
	})
}
