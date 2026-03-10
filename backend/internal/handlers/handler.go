package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/endophage/aiexplains/backend/internal/ai"
	"github.com/endophage/aiexplains/backend/internal/db"
)

type Handler struct {
	db      *db.DB
	dataDir string
	ai      *ai.Client
}

func New(database *db.DB, dataDir string, localExec bool) *Handler {
	return &Handler{
		db:      database,
		dataDir: dataDir,
		ai:      ai.NewClient(localExec),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/explanations", h.ListExplanations)
	mux.HandleFunc("POST /api/explanations", h.CreateExplanation)
	mux.HandleFunc("GET /api/explanations/{id}", h.GetExplanation)
	mux.HandleFunc("PATCH /api/explanations/{id}", h.PatchExplanation)
	mux.HandleFunc("DELETE /api/explanations/{id}", h.DeleteExplanation)
	mux.HandleFunc("POST /api/explanations/{id}/regenerate", h.RegenerateExplanation)
	mux.HandleFunc("POST /api/explanations/{id}/sections/{sectionId}/explain", h.ExplainSection)
	mux.HandleFunc("POST /api/explanations/{id}/sections/{sectionId}/extend", h.ExtendSection)
	mux.HandleFunc("DELETE /api/explanations/{id}/sections/{sectionId}", h.DeleteSection)
	mux.HandleFunc("POST /api/explanations/{id}/sections/{sectionId}/restore", h.RestoreSection)
	mux.HandleFunc("POST /api/explanations/{id}/reorder", h.ReorderSections)
	mux.HandleFunc("GET /api/tags", h.ListTags)
	mux.HandleFunc("DELETE /api/tags/{tag}", h.DeleteTag)
	mux.HandleFunc("POST /api/explanations/{id}/tags", h.AddExplanationTag)
	mux.HandleFunc("DELETE /api/explanations/{id}/tags/{tag}", h.RemoveExplanationTag)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
