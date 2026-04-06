package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"wabridge/internal/action"
	"wabridge/internal/feature"
)

// APIServer exposes a REST API that proxies actions to an action.Backend.
// It is used in bridge mode so that a separate MCP client process can
// send WhatsApp actions over HTTP.
type APIServer struct {
	backend  action.Backend
	addr     string
	features feature.Config
}

// NewAPIServer creates a new APIServer bound to the given address.
func NewAPIServer(backend action.Backend, addr string, features feature.Config) *APIServer {
	return &APIServer{
		backend:  backend,
		addr:     addr,
		features: features,
	}
}

// Handler returns the HTTP handler with all routes registered.
func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/features", s.handleFeatures)

	if s.features.Send {
		mux.HandleFunc("POST /api/send", s.handleSend)
		mux.HandleFunc("POST /api/send-file", s.handleSendFile)
		mux.HandleFunc("POST /api/send-audio", s.handleSendAudio)
	}
	if s.features.Download {
		mux.HandleFunc("POST /api/download", s.handleDownload)
	}
	if s.features.HistorySync {
		mux.HandleFunc("POST /api/sync-history", s.handleSyncHistory)
	}
	return mux
}

// Start begins serving HTTP requests. Blocks until shutdown or fatal error.
func (s *APIServer) Start() error {
	return http.ListenAndServe(s.addr, s.Handler())
}

// apiResponse is the standard JSON envelope for all API responses.
type apiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// --- Handlers ---------------------------------------------------------------

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "ok",
	})
}

func (s *APIServer) handleFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    s.features,
	})
}

func (s *APIServer) handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		Message   string `json:"message"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.Recipient == "" || req.Message == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: "recipient and message are required",
		})
		return
	}

	if err := s.backend.SendMessage(r.Context(), req.Recipient, req.Message); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Message: fmt.Sprintf("failed to send message: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "sent",
	})
}

func (s *APIServer) handleSendFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		FilePath  string `json:"file_path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.Recipient == "" || req.FilePath == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: "recipient and file_path are required",
		})
		return
	}

	if err := s.backend.SendFile(r.Context(), req.Recipient, req.FilePath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Message: fmt.Sprintf("failed to send file: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "file sent",
	})
}

func (s *APIServer) handleSendAudio(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		FilePath  string `json:"file_path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.Recipient == "" || req.FilePath == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: "recipient and file_path are required",
		})
		return
	}

	if err := s.backend.SendAudioMessage(r.Context(), req.Recipient, req.FilePath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Message: fmt.Sprintf("failed to send audio: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "audio sent",
	})
}

func (s *APIServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MessageID string `json:"message_id"`
		ChatJID   string `json:"chat_jid"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.MessageID == "" || req.ChatJID == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: "message_id and chat_jid are required",
		})
		return
	}

	path, err := s.backend.DownloadMedia(r.Context(), req.MessageID, req.ChatJID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Message: fmt.Sprintf("failed to download media: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    map[string]string{"path": path},
	})
}

func (s *APIServer) handleSyncHistory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatJID string `json:"chat_jid"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.ChatJID == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Message: "chat_jid is required",
		})
		return
	}

	if err := s.backend.RequestHistorySync(r.Context(), req.ChatJID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Message: fmt.Sprintf("failed to request history sync: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "history sync requested",
	})
}

// --- Helpers ----------------------------------------------------------------

// readJSON decodes the request body as JSON into v.
func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("json decode: %w", err)
	}
	return nil
}

// writeJSON encodes v as JSON and writes it to the response with the given
// HTTP status code and Content-Type application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
