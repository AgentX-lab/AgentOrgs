package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/collaboration"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	matrixprovider "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/collaboration/matrix"
)

// HTTPServer exposes collaboration APIs and Matrix AppService endpoints.
type HTTPServer struct {
	Engine     *collaboration.Engine
	AppService *matrixprovider.AppServiceHandler
	Addr       string
}

func (s *HTTPServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/v1/collaborations/", s.handleCollaborations)
	mux.HandleFunc("/api/v1/runs/", s.handleRuns)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	if s.AppService != nil {
		mux.HandleFunc("/_matrix/app/v1/transactions/", s.AppService.HandleTransactions)
	}
	return http.ListenAndServe(s.Addr, mux)
}

type startCollaborationRequest struct {
	From    string                  `json:"from"`
	To      []protocol.ObjectTarget `json:"to"`
	Payload map[string]interface{}  `json:"payload"`
}

func (s *HTTPServer) handleCollaborations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/runs") {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		http.NotFound(w, r)
		return
	}
	namespace := parts[2]
	name := parts[3]

	var req startCollaborationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	run, err := s.Engine.StartCollaboration(r.Context(), namespace, name, req.From, req.To, req.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, run)
}

func (s *HTTPServer) handleRuns(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "agentorgs"
	}
	runID := parts[3]
	if len(parts) == 5 && parts[4] == "events" {
		events, err := s.Engine.ListCollaborationMessages(r.Context(), namespace, runID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, events)
		return
	}
	run, err := s.Engine.GetCollaborationRun(r.Context(), namespace, runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, run)
}

func (s *HTTPServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var event protocol.CollaborationEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	run, err := s.Engine.ReceiveMessage(r.Context(), event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, run)
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
