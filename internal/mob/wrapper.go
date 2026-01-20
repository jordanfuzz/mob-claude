package mob

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Wrapper provides methods to interact with the mob.sh CLI
type Wrapper struct {
	mobPath string
}

// NewWrapper creates a new mob.sh wrapper
func NewWrapper() *Wrapper {
	// Try to find mob in PATH
	mobPath, err := exec.LookPath("mob")
	if err != nil {
		mobPath = "mob" // Will fail at runtime if not found
	}
	return &Wrapper{mobPath: mobPath}
}

// Start executes 'mob start' with the given branch name
func (w *Wrapper) Start(branch string) error {
	args := []string{"start"}
	if branch != "" {
		args = append(args, branch)
	}
	return w.runPassthrough(args...)
}

// Next executes 'mob next' to hand off to the next driver
func (w *Wrapper) Next() error {
	return w.runPassthrough("next")
}

// Done executes 'mob done' to squash commits and complete the mob session
func (w *Wrapper) Done() error {
	return w.runPassthrough("done")
}

// Status executes 'mob status' and returns the output
func (w *Wrapper) Status() (string, error) {
	return w.runCapture("status")
}

// GetCurrentBranch returns the current git branch name
func (w *Wrapper) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRepoURL returns the remote URL for the repository
func (w *Wrapper) GetRepoURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repo URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDiffSinceLastCommit returns the git diff for staged and unstaged changes
func (w *Wrapper) GetDiffSinceLastCommit() (string, error) {
	// Get both staged and unstaged changes
	cmd := exec.Command("git", "diff", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		// If HEAD doesn't exist (new repo), try without it
		cmd = exec.Command("git", "diff")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get diff: %w", err)
		}
	}
	return string(output), nil
}

// GetDiffFromBase returns the diff from the base branch (usually main/master)
func (w *Wrapper) GetDiffFromBase() (string, error) {
	// Try to find merge-base with main or master
	bases := []string{"origin/main", "origin/master", "main", "master"}

	for _, base := range bases {
		cmd := exec.Command("git", "merge-base", base, "HEAD")
		mergeBase, err := cmd.Output()
		if err != nil {
			continue
		}

		// Get diff from merge-base
		cmd = exec.Command("git", "diff", strings.TrimSpace(string(mergeBase)))
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		return string(output), nil
	}

	// Fallback to just the last commit diff
	return w.GetDiffSinceLastCommit()
}

// GetRecentCommits returns recent commit messages on the current branch
func (w *Wrapper) GetRecentCommits(count int) (string, error) {
	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", count), "--oneline")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get recent commits: %w", err)
	}
	return string(output), nil
}

// IsMobBranch checks if the current branch is a mob session branch
func (w *Wrapper) IsMobBranch() (bool, error) {
	branch, err := w.GetCurrentBranch()
	if err != nil {
		return false, err
	}
	// Mob branches typically have a wip/ prefix or -wip suffix
	return strings.HasPrefix(branch, "mob/") ||
		strings.HasSuffix(branch, "-wip") ||
		strings.Contains(branch, "/mob-"), nil
}

// GetBaseBranch extracts the base branch name from a mob branch
// e.g., "mob/feature-auth" -> "feature-auth", "feature-auth-wip" -> "feature-auth"
func (w *Wrapper) GetBaseBranch() (string, error) {
	branch, err := w.GetCurrentBranch()
	if err != nil {
		return "", err
	}

	// Remove mob/ prefix if present
	branch = strings.TrimPrefix(branch, "mob/")
	// Remove -wip suffix if present
	branch = strings.TrimSuffix(branch, "-wip")

	return branch, nil
}

// runPassthrough runs a mob command with output going directly to stdout/stderr
func (w *Wrapper) runPassthrough(args ...string) error {
	cmd := exec.Command(w.mobPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// runCapture runs a mob command and captures its output
func (w *Wrapper) runCapture(args ...string) (string, error) {
	cmd := exec.Command(w.mobPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("mob %s failed: %w\n%s", args[0], err, stderr.String())
	}

	return stdout.String(), nil
}

// CheckMobInstalled verifies that mob.sh is available
func (w *Wrapper) CheckMobInstalled() error {
	cmd := exec.Command(w.mobPath, "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mob.sh is not installed or not in PATH. Install from: https://mob.sh")
	}
	return nil
}
