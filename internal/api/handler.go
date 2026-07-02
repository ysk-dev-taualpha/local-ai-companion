package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
)

type PythonClient interface {
	Send(req client.ConversationRequest) (*client.ConversationResponse, error)
}

type Handler struct {
	PythonClient PythonClient
}

func New(pythonClient PythonClient) *Handler {
	return &Handler{PythonClient: pythonClient}
}

func (h *Handler) HandleConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": map[string]string{"code": "method_not_allowed", "message": "only POST is allowed"},
		})
		return
	}

	var req client.ConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]string{"code": "invalid_request", "message": "invalid JSON body"},
		})
		return
	}

	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]string{"code": "invalid_request", "message": "message is required"},
		})
		return
	}
	if req.RequestID == "" {
		req.RequestID = newRequestID()
	}

	resp, err := h.PythonClient.Send(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": map[string]string{"code": "python_service_error", "message": err.Error()},
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
