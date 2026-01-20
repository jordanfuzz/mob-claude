package summary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mob-claude/mob-claude/internal/plans"
)

// Generator handles AI-powered summary generation using Claude CLI
type Generator struct {
	model    string
	maxTurns int
}

// NewGenerator creates a new summary generator
func NewGenerator(model string, maxTurns int) *Generator {
	return &Generator{
		model:    model,
		maxTurns: maxTurns,
	}
}

// GeneratedSummary is the structured output from Claude
type GeneratedSummary struct {
	TLDR      string   `json:"tldr"`
	Changes   []string `json:"changes"`
	NextSteps []string `json:"nextSteps"`
}

// Generate creates a summary using Claude CLI
func (g *Generator) Generate(diff string, driverNote string, branch string) (*plans.Summary, error) {
	prompt := g.buildPrompt(diff, driverNote)

	// Call Claude CLI with structured output
	result, err := g.callClaude(prompt)
	if err != nil {
		// Return a basic summary if Claude fails
		return g.fallbackSummary(driverNote, branch), nil
	}

	// Parse the structured output
	generated, err := g.parseResponse(result)
	if err != nil {
		return g.fallbackSummary(driverNote, branch), nil
	}

	return &plans.Summary{
		Timestamp:  time.Now(),
		DriverName: "", // Will be set by caller
		DriverNote: driverNote,
		TLDR:       generated.TLDR,
		Changes:    generated.Changes,
		NextSteps:  generated.NextSteps,
		Branch:     branch,
	}, nil
}

func (g *Generator) buildPrompt(diff string, driverNote string) string {
	// Truncate diff if too long
	maxDiffLen := 10000
	if len(diff) > maxDiffLen {
		diff = diff[:maxDiffLen] + "\n... (truncated)"
	}

	return fmt.Sprintf(`Analyze this git diff from a mob programming rotation and create a brief summary.

Driver's note: %s

Git diff:
%s

Return a JSON object with:
- tldr: One sentence summary of what was accomplished (max 100 chars)
- changes: Array of 2-4 specific changes made
- nextSteps: Array of 1-3 suggested next steps for the next driver

Respond ONLY with valid JSON, no markdown or explanation.`, driverNote, diff)
}

func (g *Generator) callClaude(prompt string) (string, error) {
	// Check if claude CLI is available
	_, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH")
	}

	args := []string{
		"-p", prompt,
		"--model", g.model,
		"--max-turns", fmt.Sprintf("%d", g.maxTurns),
		"--output-format", "text",
	}

	cmd := exec.Command("claude", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

func (g *Generator) parseResponse(response string) (*GeneratedSummary, error) {
	// Try to extract JSON from the response
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Find JSON object in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	jsonStr := response[start : end+1]

	var summary GeneratedSummary
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &summary, nil
}

func (g *Generator) fallbackSummary(driverNote string, branch string) *plans.Summary {
	tldr := "Rotation completed"
	if driverNote != "" {
		// Use driver note as TLDR if provided
		if len(driverNote) > 100 {
			tldr = driverNote[:97] + "..."
		} else {
			tldr = driverNote
		}
	}

	return &plans.Summary{
		Timestamp:  time.Now(),
		DriverNote: driverNote,
		TLDR:       tldr,
		Changes:    []string{"Changes made during rotation"},
		NextSteps:  []string{"Continue from where the previous driver left off"},
		Branch:     branch,
	}
}

// CheckClaudeAvailable verifies that the Claude CLI is installed
func CheckClaudeAvailable() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found. Install from: https://claude.ai/code")
	}

	// Try running claude --version
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude CLI found but not working: %w", err)
	}

	return nil
}
