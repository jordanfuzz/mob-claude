package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	PlansDir     = ".claude/plans"
	SummariesDir = ".claude/mob/summaries"
)

// Manager handles plan file operations
type Manager struct {
	projectRoot string
}

// NewManager creates a new plan manager for the current project
func NewManager() (*Manager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	return &Manager{projectRoot: cwd}, nil
}

// GetPlanPath returns the path to the plan file for a given branch
func (m *Manager) GetPlanPath(branch string) string {
	// Sanitize branch name for filename
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	safeBranch = strings.ReplaceAll(safeBranch, "\\", "-")
	filename := fmt.Sprintf("mob-%s.md", safeBranch)
	return filepath.Join(m.projectRoot, PlansDir, filename)
}

// GetSummariesDir returns the path to the summaries directory
func (m *Manager) GetSummariesDir() string {
	return filepath.Join(m.projectRoot, SummariesDir)
}

// EnsureDirs creates the necessary directories if they don't exist
func (m *Manager) EnsureDirs() error {
	dirs := []string{
		filepath.Join(m.projectRoot, PlansDir),
		filepath.Join(m.projectRoot, SummariesDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// LoadPlan reads the plan file for a given branch
func (m *Manager) LoadPlan(branch string) (string, error) {
	planPath := m.GetPlanPath(branch)
	data, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No plan yet
		}
		return "", fmt.Errorf("failed to read plan: %w", err)
	}
	return string(data), nil
}

// SavePlan writes the plan file for a given branch
func (m *Manager) SavePlan(branch, content string) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	planPath := m.GetPlanPath(branch)
	if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}
	return nil
}

// PlanExists checks if a plan file exists for the given branch
func (m *Manager) PlanExists(branch string) bool {
	planPath := m.GetPlanPath(branch)
	_, err := os.Stat(planPath)
	return err == nil
}

// CreateDefaultPlan creates a new plan file with a template
func (m *Manager) CreateDefaultPlan(branch string) error {
	template := fmt.Sprintf(`# Mob Session: %s

## Goal
_Describe the goal of this mob session_

## Current Status
- [ ] Task 1
- [ ] Task 2

## Notes
_Add notes during the session_

## Decisions Made
_Document important decisions_

---
Created: %s
`, branch, time.Now().Format(time.RFC3339))

	return m.SavePlan(branch, template)
}

// Summary represents a rotation summary
type Summary struct {
	Timestamp  time.Time `json:"timestamp"`
	DriverName string    `json:"driverName"`
	DriverNote string    `json:"driverNote"`
	TLDR       string    `json:"tldr"`
	Changes    []string  `json:"changes"`
	NextSteps  []string  `json:"nextSteps"`
	Branch     string    `json:"branch"`
}

// SaveSummary writes a summary to the summaries directory
func (m *Manager) SaveSummary(summary *Summary) error {
	if err := m.EnsureDirs(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.json", summary.Timestamp.Format("2006-01-02T15-04-05"))
	summaryPath := filepath.Join(m.GetSummariesDir(), filename)

	// Format as JSON manually to avoid import cycle
	content := fmt.Sprintf(`{
  "timestamp": "%s",
  "driverName": "%s",
  "driverNote": "%s",
  "tldr": "%s",
  "changes": [%s],
  "nextSteps": [%s],
  "branch": "%s"
}`,
		summary.Timestamp.Format(time.RFC3339),
		escapeJSON(summary.DriverName),
		escapeJSON(summary.DriverNote),
		escapeJSON(summary.TLDR),
		formatStringArray(summary.Changes),
		formatStringArray(summary.NextSteps),
		escapeJSON(summary.Branch),
	)

	return os.WriteFile(summaryPath, []byte(content), 0644)
}

// ListSummaries returns all summaries in chronological order
func (m *Manager) ListSummaries() ([]string, error) {
	summariesDir := m.GetSummariesDir()
	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, filepath.Join(summariesDir, entry.Name()))
		}
	}
	return files, nil
}

// GetLatestSummary returns the most recent summary file content
func (m *Manager) GetLatestSummary() (string, error) {
	files, err := m.ListSummaries()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}

	// Files are sorted by name (timestamp), so last is newest
	latestPath := files[len(files)-1]
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Helper functions
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func formatStringArray(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	var quoted []string
	for _, s := range arr {
		quoted = append(quoted, fmt.Sprintf("\"%s\"", escapeJSON(s)))
	}
	return strings.Join(quoted, ", ")
}
