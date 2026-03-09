package ai

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type Message struct {
	Role    string
	Content string
}

// Client handles AI requests via either the Anthropic SDK or the local `claude` CLI.
type Client struct {
	sdkClient *anthropic.Client
	localExec bool
}

func NewClient(localExec bool) *Client {
	c := &Client{localExec: localExec}
	if !localExec {
		c.sdkClient = anthropic.NewClient()
	}
	return c
}

// GenerateExplanation asks Claude to produce a title and multi-section HTML explanation for a topic.
// Returns (title, sectionsHTML, error).
func (c *Client) GenerateExplanation(ctx context.Context, topic string) (string, string, error) {
	prompt := fmt.Sprintf(`Generate a comprehensive HTML explanation of the following topic: %s

Requirements:
1. First, output a concise and descriptive title as: <h1>Your Title Here</h1>
   The title should be clear and specific — not just the raw topic string.
2. Then output 3-6 HTML sections covering different aspects of the topic.
3. Each section MUST follow this exact format:

<div class="section" id="section-{slug}" data-current-version="1">
<div class="section-version" data-version="1">
<h2>Section Title</h2>
<p>Content...</p>
</div>
</div>

4. Use descriptive kebab-case section IDs (e.g., section-overview, section-key-concepts, section-examples).
5. Use appropriate HTML elements: h2 for section titles, p for paragraphs, ul/ol for lists, code/pre for code samples.
6. Do NOT include <!DOCTYPE>, <html>, <head>, or <body> tags.
7. Return ONLY the h1 title followed by the HTML sections. No markdown, no code fences, no text outside of HTML.`, topic)

	var raw string
	var err error
	if c.localExec {
		raw, err = c.execClaude(ctx, prompt)
	} else {
		raw, err = c.sdkGenerate(ctx, prompt)
	}
	if err != nil {
		return "", "", err
	}

	title, sectionsHTML := splitTitleFromSections(raw)
	return title, sectionsHTML, nil
}

// splitTitleFromSections splits AI output into title and sections HTML.
func splitTitleFromSections(content string) (title, sections string) {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "<h1")
	if start == -1 {
		return "", content
	}
	closeTag := strings.Index(content[start:], ">")
	if closeTag == -1 {
		return "", content
	}
	innerStart := start + closeTag + 1
	endTag := strings.Index(strings.ToLower(content[innerStart:]), "</h1>")
	if endTag == -1 {
		return "", content
	}
	title = strings.TrimSpace(content[innerStart : innerStart+endTag])
	sections = strings.TrimSpace(content[innerStart+endTag+5:])
	return
}

// GenerateNewSection asks Claude to produce one or more new HTML sections to follow an existing one.
func (c *Client) GenerateNewSection(ctx context.Context, topic, afterSectionContent, userPrompt string, existingIDs []string) (string, error) {
	prompt := fmt.Sprintf(`You are adding new content to an HTML explanation about "%s".

The new content should follow this existing section:
---
%s
---

The user wants to cover: %s

Existing section IDs (do NOT reuse these): %s

Requirements:
1. Generate one or more new HTML sections. If the requested content naturally spans multiple distinct topics, create multiple sections.
2. Each section MUST follow this exact format:

<div class="section" id="section-{slug}" data-current-version="1">
<div class="section-version" data-version="1">
<h2>Section Title</h2>
<p>Content...</p>
</div>
</div>

3. Use unique descriptive kebab-case section IDs that are NOT in the existing IDs list.
4. Use appropriate HTML elements: h2 for section titles, p for paragraphs, ul/ol for lists, code/pre for code.
5. Do NOT include <!DOCTYPE>, <html>, <head>, or <body> tags.
6. Return ONLY the HTML sections. No markdown, no code fences, no text outside of HTML.`,
		topic, afterSectionContent, userPrompt, strings.Join(existingIDs, ", "))

	if c.localExec {
		return c.execClaude(ctx, prompt)
	}
	return c.sdkGenerate(ctx, prompt)
}

// ExpandSection asks Claude to produce an expanded version of a section.
// history is the prior conversation for this section (may be empty for first expansion).
func (c *Client) ExpandSection(ctx context.Context, topic, sectionContent, userPrompt string, history []Message) (string, error) {
	system := `You are an expert educator providing detailed HTML explanations.
When asked to explain or expand a section, you have two options:

Option A (single section expansion): Return ONLY the inner HTML content — no wrapper divs.
Use h2 for the section title, p for paragraphs, ul/ol for lists, code/pre for code.

Option B (multiple sections): If the answer naturally splits into multiple distinct sub-topics,
return two or more complete section divs in this format:
<div class="section" id="section-{slug}" data-current-version="1">
<div class="section-version" data-version="1">
<h2>Title</h2>
<p>Content</p>
</div>
</div>

Choose whichever option best serves the user's question. No markdown, no code fences. Return only valid HTML.`

	if c.localExec {
		return c.execExpandSection(ctx, system, topic, sectionContent, userPrompt, history)
	}
	return c.sdkExpandSection(ctx, system, topic, sectionContent, userPrompt, history)
}

// execClaude runs the local `claude` CLI with the given prompt via stdin.
func (c *Client) execClaude(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stdoutStr := strings.TrimSpace(stdout.String())
		stderrStr := strings.TrimSpace(stderr.String())
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with status %d\nstdout: %q\nstderr: %q", exitErr.ExitCode(), stdoutStr, stderrStr)
		}
		return "", fmt.Errorf("running claude: %w\nstdout: %q\nstderr: %q", err, stdoutStr, stderrStr)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// execExpandSection builds a single prompt encoding history and calls claude -p.
func (c *Client) execExpandSection(ctx context.Context, system, topic, sectionContent, userPrompt string, history []Message) (string, error) {
	var sb strings.Builder
	sb.WriteString(system)
	sb.WriteString("\n\n")

	if len(history) == 0 {
		sb.WriteString(fmt.Sprintf(
			"I'm reading an explanation about %q. Here is the current content of a section:\n\n---\n%s\n---\n\nPlease provide a more detailed and thorough version of this section. %s",
			topic, sectionContent, userPrompt,
		))
	} else {
		// Replay conversation history as a transcript, then append the new request.
		sb.WriteString("Previous conversation:\n")
		for _, m := range history {
			role := "User"
			if m.Role == "assistant" {
				role = "Assistant"
			}
			sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", role, m.Content))
		}
		sb.WriteString(fmt.Sprintf("\nUser:\n%s", userPrompt))
	}

	return c.execClaude(ctx, sb.String())
}

// --- SDK implementations ---

func (c *Client) sdkGenerate(ctx context.Context, prompt string) (string, error) {
	msg, err := c.sdkClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.ModelClaude3_5SonnetLatest),
		MaxTokens: anthropic.F(int64(8192)),
		Messages: anthropic.F([]anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		}),
	})
	if err != nil {
		return "", fmt.Errorf("calling claude sdk: %w", err)
	}
	return extractText(msg)
}

func (c *Client) sdkExpandSection(ctx context.Context, system, topic, sectionContent, userPrompt string, history []Message) (string, error) {
	var messages []anthropic.MessageParam

	if len(history) == 0 {
		firstMsg := fmt.Sprintf(
			"I'm reading an explanation about %q. Here is the current content of a section:\n\n---\n%s\n---\n\nPlease provide a more detailed and thorough version of this section. %s",
			topic, sectionContent, userPrompt,
		)
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(firstMsg)))
	} else {
		for _, m := range history {
			switch m.Role {
			case "user":
				messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
			case "assistant":
				messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
			}
		}
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)))
	}

	msg, err := c.sdkClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.ModelClaude3_5SonnetLatest),
		MaxTokens: anthropic.F(int64(8192)),
		System: anthropic.F([]anthropic.TextBlockParam{
			{
				Type: anthropic.F(anthropic.TextBlockParamTypeText),
				Text: anthropic.F(system),
			},
		}),
		Messages: anthropic.F(messages),
	})
	if err != nil {
		return "", fmt.Errorf("calling claude sdk: %w", err)
	}
	return extractText(msg)
}

func extractText(msg *anthropic.Message) (string, error) {
	for _, block := range msg.Content {
		if block.Type == anthropic.ContentBlockTypeText {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}
