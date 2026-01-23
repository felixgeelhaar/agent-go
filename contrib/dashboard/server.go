// Package dashboard provides a web-based monitoring dashboard for agent-go.
//
// The dashboard enables real-time monitoring of agent runs, including:
//   - Active run list with status indicators
//   - Real-time event streaming via WebSockets
//   - Run history and search
//   - Evidence and decision visualization
//   - Budget and constraint status
//
// # Usage
//
//	srv := dashboard.New(dashboard.Config{
//		EventStore: myEventStore,
//		RunStore:   myRunStore,
//		Address:    ":8080",
//	})
//
//	if err := srv.Start(); err != nil {
//		log.Fatal(err)
//	}
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// Config configures the dashboard server.
type Config struct {
	// EventStore provides access to agent events.
	EventStore event.Store

	// RunStore provides access to run state.
	RunStore run.Store

	// Address is the HTTP listen address (default ":8080").
	Address string

	// BasePath is the URL path prefix (default "/").
	BasePath string

	// StaticDir is the path to static assets (optional).
	// If empty, embedded assets are used.
	StaticDir string

	// EnableCORS enables Cross-Origin Resource Sharing.
	EnableCORS bool

	// ReadTimeout is the HTTP read timeout.
	ReadTimeout time.Duration

	// WriteTimeout is the HTTP write timeout.
	WriteTimeout time.Duration
}

// Server is the dashboard HTTP server.
type Server struct {
	config     Config
	httpServer *http.Server
	mux        *http.ServeMux
	clients    map[string]map[chan event.Event]struct{} // runID -> clients
	mu         sync.RWMutex
}

// New creates a new dashboard server.
func New(cfg Config) *Server {
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "/"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}

	s := &Server{
		config:  cfg,
		mux:     http.NewServeMux(),
		clients: make(map[string]map[chan event.Event]struct{}),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() {
	base := s.config.BasePath

	// API routes
	s.mux.HandleFunc(base+"api/runs", s.handleListRuns)
	s.mux.HandleFunc(base+"api/runs/", s.handleGetRun)
	s.mux.HandleFunc(base+"api/runs/events", s.handleRunEvents)
	s.mux.HandleFunc(base+"api/health", s.handleHealth)

	// WebSocket route for real-time updates
	s.mux.HandleFunc(base+"ws/", s.handleWebSocket)

	// Static assets
	s.mux.HandleFunc(base, s.handleIndex)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return s.httpServer.ListenAndServe()
}

// StartTLS starts the HTTPS server.
func (s *Server) StartTLS(certFile, keyFile string) error {
	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// withMiddleware wraps the handler with common middleware.
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		handler.ServeHTTP(w, r)
	})
}

// handleIndex serves the main dashboard page.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	// TODO: Serve embedded or static HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Agent Dashboard</title>
</head>
<body>
    <h1>Agent Dashboard</h1>
    <p>Dashboard implementation pending.</p>
</body>
</html>`))
}

// handleListRuns returns a list of all runs.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement run listing
	runs := []RunSummary{
		{
			ID:        "example-run-1",
			Goal:      "Example goal",
			State:     string(agent.StateIntake),
			Status:    string(agent.RunStatusRunning),
			StartTime: time.Now().Add(-5 * time.Minute),
		},
	}

	s.writeJSON(w, runs)
}

// handleGetRun returns details for a specific run.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Parse run ID from path and fetch run details
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleRunEvents returns events for a run.
func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement event fetching
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleWebSocket handles WebSocket connections for real-time updates.
func (s *Server) handleWebSocket(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement WebSocket upgrade and event streaming
	http.Error(w, "WebSocket support pending", http.StatusNotImplemented)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	health := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
	}
	s.writeJSON(w, health)
}

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RunSummary is a condensed run representation for listing.
type RunSummary struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	State     string    `json:"state"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// RunDetail is a full run representation with events.
type RunDetail struct {
	RunSummary
	Evidence []agent.Evidence `json:"evidence"`
	Events   []event.Event    `json:"events"`
	Vars     map[string]any   `json:"vars"`
}

// HealthStatus represents server health.
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}
