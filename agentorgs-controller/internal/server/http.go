package server

import (
	"encoding/json"
	"net/http"
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/collaboration"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	matrixprovider "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/collaboration/matrix"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HTTPServer exposes collaboration APIs and Matrix AppService endpoints.
type HTTPServer struct {
	Engine     *collaboration.Engine
	AppService *matrixprovider.AppServiceHandler
	Client     client.Client
	Addr       string
}

func (s *HTTPServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/v1/members/{namespace}/{name}/ready", s.handleMemberReady)
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

func (s *HTTPServer) handleMemberReady(w http.ResponseWriter, r *http.Request) {
	if s.Client == nil {
		http.Error(w, "member client is not configured", http.StatusInternalServerError)
		return
	}
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		http.NotFound(w, r)
		return
	}

	key := client.ObjectKey{Namespace: namespace, Name: name}
	for attempt := 0; attempt < 5; attempt++ {
		var member agentorgsv1alpha1.Member
		if err := s.Client.Get(r.Context(), key, &member); err != nil {
			if apierrors.IsNotFound(err) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if member.Annotations[agentorgsv1alpha1.MemberRuntimeReadyAnnotation] == "true" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		orig := member.DeepCopy()
		if member.Annotations == nil {
			member.Annotations = map[string]string{}
		}
		member.Annotations[agentorgsv1alpha1.MemberRuntimeReadyAnnotation] = "true"
		if err := s.Client.Patch(r.Context(), &member, client.MergeFrom(orig)); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "conflict patching runtime-ready annotation", http.StatusConflict)
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
