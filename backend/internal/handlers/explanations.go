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

	aiTitle, htmlContent, err := h.ai.GenerateExplanation(r.Context(), topic)
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

	title := aiTitle
	if title == "" {
		title = titleCase(topic)
	}

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

	// Check if the AI returned structured sections or raw content.
	// If sections are present, the first replaces the current section; remaining are inserted after.
	existingIDs := make(map[string]bool, len(sections))
	for _, s := range sections {
		existingIDs[s.ID] = true
	}

	parsedSections, _ := htmlutil.ParseSections(newContent)
	var versionContent string
	var extraSections []htmlutil.SectionData
	if len(parsedSections) > 0 {
		versionContent = parsedSections[0].Versions[0].Content
		extraSections = deduplicateIDs(parsedSections[1:], existingIDs)
	} else {
		versionContent = newContent
	}

	// Add new version to the existing section
	updatedSections, newVersion, err := htmlutil.AddSectionVersion(sections, sectionID, versionContent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update section")
		return
	}

	// Insert any extra sections after the updated section
	insertAfterID := sectionID
	for _, extra := range extraSections {
		updatedSections, err = htmlutil.InsertSectionAfter(updatedSections, insertAfterID, extra)
		if err != nil {
			log.Printf("WARNING InsertSectionAfter explanation=%s after=%s: %v", id, insertAfterID, err)
			break
		}
		insertAfterID = extra.ID
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, updatedSections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)

	var updatedSection *SectionResponse
	for _, s := range updatedSections {
		if s.ID == sectionID {
			versions := make([]SectionVersionResponse, len(s.Versions))
			for i, v := range s.Versions {
				versions[i] = SectionVersionResponse{Version: v.Version, Content: v.Content}
			}
			sr := SectionResponse{ID: s.ID, CurrentVersion: newVersion, Versions: versions}
			updatedSection = &sr
			break
		}
	}
	if updatedSection == nil {
		writeError(w, http.StatusInternalServerError, "unexpected: section missing after update")
		return
	}

	resp := map[string]any{"section": updatedSection}
	if len(extraSections) > 0 {
		resp["new_sections"] = sectionsToResponse(extraSections)
	}
	writeJSON(w, http.StatusOK, resp)
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

	// Deduplicate IDs across all returned sections
	taken := make(map[string]bool, len(existingIDs))
	for _, sid := range existingIDs {
		taken[sid] = true
	}
	newSections = deduplicateIDs(newSections, taken)

	// Insert all new sections after the target section (chaining afterID)
	updatedSections := sections
	insertAfterID := sectionID
	for _, ns := range newSections {
		updatedSections, err = htmlutil.InsertSectionAfter(updatedSections, insertAfterID, ns)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to insert section")
			return
		}
		insertAfterID = ns.ID
	}

	doc := htmlutil.RenderExplanation(explanation.ID, explanation.Title, updatedSections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}
	h.db.TouchExplanation(id)

	writeJSON(w, http.StatusCreated, map[string]any{
		"sections": sectionsToResponse(newSections),
	})
}

func (h *Handler) PatchExplanation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	title := strings.TrimSpace(body.Title)

	explanation, err := h.db.GetExplanation(id)
	if err != nil || explanation == nil {
		writeError(w, http.StatusNotFound, "explanation not found")
		return
	}

	if err := h.db.UpdateExplanationTitle(id, title); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update title")
		return
	}

	// Re-render HTML file with new title
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
	doc := htmlutil.RenderExplanation(explanation.ID, title, sections)
	if err := os.WriteFile(explanation.FilePath, []byte(doc), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write updated explanation")
		return
	}

	explanation.Title = title
	writeJSON(w, http.StatusOK, toExplanationResponse(explanation, nil))
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

// deduplicateIDs ensures all sections have unique IDs, resolving conflicts against taken.
// taken is updated in-place as new IDs are assigned.
func deduplicateIDs(sections []htmlutil.SectionData, taken map[string]bool) []htmlutil.SectionData {
	result := make([]htmlutil.SectionData, len(sections))
	for i, s := range sections {
		if taken[s.ID] {
			base := s.ID
			for j := 2; ; j++ {
				candidate := fmt.Sprintf("%s-%d", base, j)
				if !taken[candidate] {
					s.ID = candidate
					break
				}
			}
		}
		taken[s.ID] = true
		result[i] = s
	}
	return result
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
