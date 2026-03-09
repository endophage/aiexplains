package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	aiClient "github.com/endophage/aiexplains/backend/internal/ai"
	"github.com/endophage/aiexplains/backend/internal/db"
	"github.com/endophage/aiexplains/backend/internal/htmlutil"
)

// API response types

type SectionVersionResponse struct {
	Version int    `json:"version"`
	Content string `json:"content"`
}

type SectionResponse struct {
	ID             string                   `json:"id"`
	CurrentVersion int                      `json:"current_version"`
	Deleted        bool                     `json:"deleted"`
	Versions       []SectionVersionResponse `json:"versions"`
}

type ExplanationResponse struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Topic     string            `json:"topic"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Sections  []SectionResponse `json:"sections,omitempty"`
}

func toExplanationResponse(e *db.Explanation, sections []SectionResponse) ExplanationResponse {
	return ExplanationResponse{
		ID:        e.ID,
		Title:     e.Title,
		Topic:     e.Topic,
		CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Sections:  sections,
	}
}

func sectionsToResponse(sections []htmlutil.SectionData) []SectionResponse {
	resp := make([]SectionResponse, len(sections))
	for i, s := range sections {
		versions := make([]SectionVersionResponse, len(s.Versions))
		for j, v := range s.Versions {
			versions[j] = SectionVersionResponse{Version: v.Version, Content: v.Content}
		}
		resp[i] = SectionResponse{
			ID:             s.ID,
			CurrentVersion: s.CurrentVersion,
			Deleted:        s.Deleted,
			Versions:       versions,
		}
	}
	return resp
}

func (h *Handler) ListExplanations(w http.ResponseWriter, r *http.Request) {
	explanations, err := h.db.ListExplanations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list explanations")
		return
	}

	resp := make([]ExplanationResponse, len(explanations))
	for i, e := range explanations {
		e := e
		resp[i] = toExplanationResponse(&e, nil)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateExplanation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Topic string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Topic) == "" {
		writeError(w, http.StatusBadRequest, "topic is required")
		return
	}
	topic := strings.TrimSpace(body.Topic)

	htmlContent, err := h.ai.GenerateExplanation(r.Context(), topic)
	if err != nil {
		log.Printf("ERROR GenerateExplanation topic=%q: %v", topic, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generating explanation: %v", err))
		return
	}

	sections, err := htmlutil.ParseSections(htmlContent)
	if err != nil || len(sections) == 0 {
		log.Printf("ERROR ParseSections (topic=%q): err=%v sections=%d raw=%q", topic, err, len(sections), htmlContent[:min(200, len(htmlContent))])
		writeError(w, http.StatusInternalServerError, "failed to parse generated explanation")
		return
	}

	title := titleCase(topic)

	explanation, err := h.db.CreateExplanation(title, topic, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save explanation")
		return
	}

	fullPath := filepath.Join(h.dataDir, "explanations", fmt.Sprintf("%s.html", explanation.ID))
	doc := htmlutil.RenderExplanation(explanation.ID, title, sections)
	if err := os.WriteFile(fullPath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write explanation file")
		return
	}

	h.db.Exec(`UPDATE explanations SET file_path = ? WHERE id = ?`, fullPath, explanation.ID)
	explanation.FilePath = fullPath

	writeJSON(w, http.StatusCreated, toExplanationResponse(explanation, sectionsToResponse(sections)))
}

func (h *Handler) GetExplanation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	writeJSON(w, http.StatusOK, toExplanationResponse(explanation, sectionsToResponse(sections)))
}

func (h *Handler) ExplainSection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sectionID := r.PathValue("sectionId")

	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Prompt) == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	// Find current section content
	var currentContent string
	for _, s := range sections {
		if s.ID == sectionID {
			for _, v := range s.Versions {
				if v.Version == s.CurrentVersion {
					currentContent = v.Content
					break
				}
			}
			break
		}
	}
	if currentContent == "" {
		writeError(w, http.StatusNotFound, "section not found")
		return
	}

	thread, err := h.db.GetOrCreateSectionThread(id, sectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get thread")
		return
	}

	// Convert DB messages to AI messages
	aiMsgs := make([]aiClient.Message, len(thread.Messages))
	for i, m := range thread.Messages {
		aiMsgs[i] = aiClient.Message{Role: m.Role, Content: m.Content}
	}

	newContent, err := h.ai.ExpandSection(r.Context(), explanation.Topic, currentContent, body.Prompt, aiMsgs)
	if err != nil {
		log.Printf("ERROR ExpandSection explanation=%s section=%s: %v", id, sectionID, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generating expansion: %v", err))
		return
	}

	// Build the first user message content (matches what ExpandSection sends when history is empty)
	var userMsgContent string
	if len(thread.Messages) == 0 {
		userMsgContent = fmt.Sprintf(`I'm reading an explanation about "%s". Here is the current content of a section:

---
%s
---

Please provide a more detailed and thorough version of this section. %s`, explanation.Topic, currentContent, body.Prompt)
	} else {
		userMsgContent = body.Prompt
	}

	// Save messages to thread
	newMessages := append(thread.Messages,
		db.Message{Role: "user", Content: userMsgContent},
		db.Message{Role: "assistant", Content: newContent},
	)
	if err := h.db.UpdateThreadMessages(thread.ID, newMessages); err != nil {
		log.Printf("WARNING UpdateThreadMessages thread=%s: %v", thread.ID, err)
	}

	// Add new version to sections and write file
	updatedSections, newVersion, err := htmlutil.AddSectionVersion(sections, sectionID, newContent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update section")
		return
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, updatedSections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)

	for _, s := range updatedSections {
		if s.ID == sectionID {
			versions := make([]SectionVersionResponse, len(s.Versions))
			for i, v := range s.Versions {
				versions[i] = SectionVersionResponse{Version: v.Version, Content: v.Content}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"section": SectionResponse{
					ID:             s.ID,
					CurrentVersion: newVersion,
					Versions:       versions,
				},
			})
			return
		}
	}

	writeError(w, http.StatusInternalServerError, "unexpected: section missing after update")
}

func (h *Handler) ExtendSection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sectionID := r.PathValue("sectionId")

	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Prompt) == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	var afterContent string
	existingIDs := make([]string, 0, len(sections))
	for _, s := range sections {
		existingIDs = append(existingIDs, s.ID)
		if s.ID == sectionID {
			for _, v := range s.Versions {
				if v.Version == s.CurrentVersion {
					afterContent = v.Content
					break
				}
			}
		}
	}
	if afterContent == "" {
		writeError(w, http.StatusNotFound, "section not found")
		return
	}

	rawHTML, err := h.ai.GenerateNewSection(r.Context(), explanation.Topic, afterContent, body.Prompt, existingIDs)
	if err != nil {
		log.Printf("ERROR GenerateNewSection explanation=%s after=%s: %v", id, sectionID, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generating section: %v", err))
		return
	}

	newSections, err := htmlutil.ParseSections(rawHTML)
	if err != nil || len(newSections) == 0 {
		log.Printf("ERROR ParseSections for new section (explanation=%s): err=%v raw=%q", id, err, rawHTML[:min(200, len(rawHTML))])
		writeError(w, http.StatusInternalServerError, "failed to parse generated section")
		return
	}
	newSection := newSections[0]

	// Ensure the ID is unique
	taken := make(map[string]bool, len(existingIDs))
	for _, sid := range existingIDs {
		taken[sid] = true
	}
	if taken[newSection.ID] {
		base := newSection.ID
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s-%d", base, i)
			if !taken[candidate] {
				newSection.ID = candidate
				break
			}
		}
	}

	updatedSections, err := htmlutil.InsertSectionAfter(sections, sectionID, newSection)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to insert section")
		return
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, updatedSections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)

	versions := make([]SectionVersionResponse, len(newSection.Versions))
	for i, v := range newSection.Versions {
		versions[i] = SectionVersionResponse{Version: v.Version, Content: v.Content}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"section": SectionResponse{
			ID:             newSection.ID,
			CurrentVersion: newSection.CurrentVersion,
			Versions:       versions,
		},
	})
}

func (h *Handler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sectionID := r.PathValue("sectionId")

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	sections, err = htmlutil.DeleteSection(sections, sectionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, sections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RestoreSection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sectionID := r.PathValue("sectionId")

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	sections, err = htmlutil.RestoreSection(sections, sectionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, sections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderSections(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		SectionIDs []string `json:"section_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.SectionIDs) == 0 {
		writeError(w, http.StatusBadRequest, "section_ids is required")
		return
	}

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	data, err := os.ReadFile(explanation.FilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read explanation file")
		return
	}

	sections, err := htmlutil.ParseSections(string(data))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse explanation")
		return
	}

	sections, err = htmlutil.ReorderSections(sections, body.SectionIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, sections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)
	w.WriteHeader(http.StatusNoContent)
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
