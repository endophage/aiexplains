package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.db.ListTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *Handler) AddExplanationTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Tag) == "" {
		writeError(w, http.StatusBadRequest, "tag is required")
		return
	}
	tag := strings.ToLower(strings.TrimSpace(body.Tag))

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	tagID, err := h.db.GetOrCreateTag(tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tag")
		return
	}

	if err := h.db.AddTagToExplanation(id, tagID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add tag")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"tag": tag})
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Tag) == "" {
		writeError(w, http.StatusBadRequest, "tag is required")
		return
	}
	tag := strings.ToLower(strings.TrimSpace(body.Tag))

	if _, err := h.db.GetOrCreateTag(tag); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tag")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"tag": tag})
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if err := h.db.DeleteTag(tag); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveExplanationTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag := r.PathValue("tag")

	if err := h.db.RemoveTagFromExplanation(id, tag); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove tag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
