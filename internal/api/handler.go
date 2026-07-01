package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
)

type PythonClient interface {
	Send(ctx context.Context, req client.ConversationRequest) (*client.ConversationResponse, error)
}

type Handler struct {
	PythonClient      PythonClient
	RequestTimeoutMs  int
}

func New(pythonClient PythonClient, requestTimeoutMs int) *Handler {
	return &Handler{PythonClient: pythonClient, RequestTimeoutMs: requestTimeoutMs}
}

func (h *Handler) HandleConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method_not_allowed", "only POST is allowed"))
		return
	}

	var req client.ConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_request", "invalid JSON body"))
		return
	}

	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid_request", "message is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.RequestTimeoutMs)*time.Millisecond)
	defer cancel()

	resp, err := h.PythonClient.Send(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			writeJSON(w, http.StatusGatewayTimeout, errBody("timeout", "request timed out"))
			return
		}
		writeJSON(w, http.StatusBadGateway, errBody("python_service_error", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func errBody(code, message string) map[string]interface{} {
	return map[string]interface{}{"error": map[string]string{"code": code, "message": message}}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
