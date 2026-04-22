package htmlutil

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type VersionData struct {
	Version int
	Content string // inner HTML
}

type SectionData struct {
	ID             string
	ParentID       string // empty = root-level section
	CurrentVersion int
	Deleted        bool
	Versions       []VersionData // sorted: newest first (highest version number first)
}

// ParseSections parses an HTML document and extracts section data.
func ParseSections(htmlContent string) ([]SectionData, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var sections []SectionData
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "section") {
			section := extractSection(n)
			sections = append(sections, section)
			return // don't recurse into sections
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return sections, nil
}

func extractSection(n *html.Node) SectionData {
	s := SectionData{
		ID:       getAttr(n, "id"),
		ParentID: getAttr(n, "data-parent-id"),
		Deleted:  getAttr(n, "data-deleted") == "true",
	}
	fmt.Sscanf(getAttr(n, "data-current-version"), "%d", &s.CurrentVersion)
	if s.CurrentVersion == 0 {
		s.CurrentVersion = 1
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && hasClass(c, "section-version") {
			var v VersionData
			fmt.Sscanf(getAttr(c, "data-version"), "%d", &v.Version)
			v.Content = innerHTML(c)
			s.Versions = append(s.Versions, v)
		}
	}
	return s
}

// innerHTML returns the inner HTML content of a node (its children rendered as HTML).
func innerHTML(n *html.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		html.Render(&buf, c)
	}
	return buf.String()
}

// RenderExplanation generates a full HTML document from an explanation's sections.
func RenderExplanation(id, title string, sections []SectionData) string {
	var buf strings.Builder
	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString("<meta charset=\"UTF-8\">\n")
	buf.WriteString(fmt.Sprintf("<title>%s</title>\n", HTMLEscape(title)))
	buf.WriteString("</head>\n<body>\n")
	buf.WriteString(fmt.Sprintf("<div class=\"explanation\" data-id=\"%s\">\n", id))

	for _, section := range sections {
		extra := ""
		if section.ParentID != "" {
			extra += fmt.Sprintf(` data-parent-id="%s"`, section.ParentID)
		}
		if section.Deleted {
			extra += ` data-deleted="true"`
		}
		buf.WriteString(fmt.Sprintf(
			"<div class=\"section\" id=\"%s\" data-current-version=\"%d\"%s>\n",
			section.ID, section.CurrentVersion, extra,
		))
		for _, v := range section.Versions {
			style := ""
			if v.Version != section.CurrentVersion {
				style = " style=\"display:none\""
			}
			buf.WriteString(fmt.Sprintf(
				"<div class=\"section-version\" data-version=\"%d\"%s>\n%s\n</div>\n",
				v.Version, style, v.Content,
			))
		}
		buf.WriteString("</div>\n")
	}

	buf.WriteString("</div>\n</body>\n</html>")
	return buf.String()
}

// DeleteSection marks a section as deleted.
func DeleteSection(sections []SectionData, sectionID string) ([]SectionData, error) {
	for i, s := range sections {
		if s.ID == sectionID {
			sections[i].Deleted = true
			return sections, nil
		}
	}
	return nil, fmt.Errorf("section %q not found", sectionID)
}

// RestoreSection marks a deleted section as active again.
func RestoreSection(sections []SectionData, sectionID string) ([]SectionData, error) {
	for i, s := range sections {
		if s.ID == sectionID {
			sections[i].Deleted = false
			return sections, nil
		}
	}
	return nil, fmt.Errorf("section %q not found", sectionID)
}

// ReorderSections reorders active sections according to orderedIDs.
// orderedIDs may be a subset (e.g. one level of the tree); unspecified active sections
// retain their original relative positions, interleaved such that the reordered IDs
// occupy the same positions in the flat list as the originals did.
// Deleted sections are always appended last.
func ReorderSections(sections []SectionData, orderedIDs []string) ([]SectionData, error) {
	specified := make(map[string]bool, len(orderedIDs))
	for _, id := range orderedIDs {
		specified[id] = true
	}

	byID := make(map[string]SectionData, len(sections))
	var deleted []SectionData
	// positions (in active-only list) occupied by the specified IDs, in encounter order
	var specPositions []int
	var active []SectionData
	for _, s := range sections {
		if s.Deleted {
			deleted = append(deleted, s)
			continue
		}
		if specified[s.ID] {
			specPositions = append(specPositions, len(active))
		}
		byID[s.ID] = s
		active = append(active, s)
	}

	for _, id := range orderedIDs {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown section ID %q", id)
		}
	}
	if len(specPositions) != len(orderedIDs) {
		return nil, fmt.Errorf("section ID count mismatch")
	}

	// Place reordered sections at the original positions of the specified IDs
	result := make([]SectionData, len(active))
	copy(result, active)
	for i, pos := range specPositions {
		result[pos] = byID[orderedIDs[i]]
	}
	return append(result, deleted...), nil
}

// InsertSectionAfter inserts a new section immediately after the section with afterID.
// If afterID is empty the new section is appended at the end.
// Returns an error if afterID is non-empty and not found.
func InsertSectionAfter(sections []SectionData, afterID string, newSection SectionData) ([]SectionData, error) {
	if afterID == "" {
		return append(sections, newSection), nil
	}
	for i, s := range sections {
		if s.ID == afterID {
			result := make([]SectionData, 0, len(sections)+1)
			result = append(result, sections[:i+1]...)
			result = append(result, newSection)
			result = append(result, sections[i+1:]...)
			return result, nil
		}
	}
	return nil, fmt.Errorf("section %q not found", afterID)
}

// AddSectionVersion adds a new version to a section in the given sections slice.
// Returns the updated sections and the new version number.
func AddSectionVersion(sections []SectionData, sectionID, content string) ([]SectionData, int, error) {
	for i, s := range sections {
		if s.ID == sectionID {
			newVersion := s.CurrentVersion + 1
			newVersionData := VersionData{
				Version: newVersion,
				Content: content,
			}
			// Prepend new version (newest first)
			sections[i].Versions = append([]VersionData{newVersionData}, s.Versions...)
			sections[i].CurrentVersion = newVersion
			return sections, newVersion, nil
		}
	}
	return nil, 0, fmt.Errorf("section %q not found", sectionID)
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func HTMLEscape(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&#34;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
