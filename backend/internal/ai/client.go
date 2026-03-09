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

// GenerateExplanation asks Claude to produce a multi-section HTML explanation for a topic.
func (c *Client) GenerateExplanation(ctx context.Context, topic string) (string, error) {
	prompt := fmt.Sprintf(`Generate a comprehensive HTML explanation of the following topic: %s

Requirements:
1. Structure the explanation as 3-6 HTML sections covering different aspects of the topic.
2. Each section MUST follow this exact format:

<div class="section" id="section-{slug}" data-current-version="1">
<div class="section-version" data-version="1">
<h2>Section Title</h2>
<p>Content...</p>
</div>
</div>

3. Use descriptive kebab-case section IDs (e.g., section-overview, section-key-concepts, section-examples).
4. Use appropriate HTML elements: h2 for section titles, p for paragraphs, ul/ol for lists, code/pre for code samples.
5. Do NOT include <!DOCTYPE>, <html>, <head>, or <body> tags.
6. Return ONLY the HTML sections. No markdown, no code fences, no text outside of HTML.`, topic)

	if c.localExec {
		return c.execClaude(ctx, prompt)
	}
	return c.sdkGenerate(ctx, prompt)
}

// GenerateNewSection asks Claude to produce a single new HTML section to follow an existing one.
func (c *Client) GenerateNewSection(ctx context.Context, topic, afterSectionContent, userPrompt string, existingIDs []string) (string, error) {
	prompt := fmt.Sprintf(`You are adding a new section to an HTML explanation about "%s".

The new section should follow this existing section:
---
%s
---

The user wants the new section to cover: %s

Existing section IDs (do NOT reuse these): %s

Requirements:
1. Generate exactly ONE new HTML section.
2. It MUST follow this exact format:

<div class="section" id="section-{slug}" data-current-version="1">
<div class="section-version" data-version="1">
<h2>Section Title</h2>
<p>Content...</p>
</div>
</div>

3. Use a unique descriptive kebab-case section ID that is NOT in the existing IDs list.
4. Use appropriate HTML elements: h2 for the section title, p for paragraphs, ul/ol for lists, code/pre for code.
5. Do NOT include <!DOCTYPE>, <html>, <head>, or <body> tags.
6. Return ONLY the HTML section. No markdown, no code fences, no text outside of HTML.`,
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
When asked to explain or expand a section, return ONLY the inner HTML content that goes inside the section-version div.
Do NOT include wrapper divs like <div class="section"> or <div class="section-version">.
Use appropriate HTML: h2 for the section title, p for paragraphs, ul/ol for lists, code/pre for code.
No markdown, no code fences. Return only valid HTML content.`

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
