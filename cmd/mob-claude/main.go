package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/mob-claude/mob-claude/internal/api"
	"github.com/mob-claude/mob-claude/internal/config"
	"github.com/mob-claude/mob-claude/internal/mob"
	"github.com/mob-claude/mob-claude/internal/plans"
	"github.com/mob-claude/mob-claude/internal/summary"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	// Global flags
	skipSummary bool
	message     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mob-claude",
		Short: "Mob programming with Claude Code integration",
		Long: `mob-claude wraps mob.sh with Claude Code context management.
It manages plan files and generates AI-powered rotation summaries.`,
		Version: version,
	}

	// Start command
	startCmd := &cobra.Command{
		Use:   "start [mob-flags...]",
		Short: "Start or join a mob session",
		Long: `Wraps 'mob start', fetches the current plan, and initializes session tracking.

All arguments are passed through to mob.sh.
Example: mob-claude start -i
Example: mob-claude start -b my-feature --include-uncommitted-changes`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE:               runStart,
	}

	// Next command
	nextCmd := &cobra.Command{
		Use:   "next [-- mob-flags...]",
		Short: "Hand off to the next driver",
		Long: `Generates a summary, uploads to the dashboard, then runs 'mob next'.

Use -- to pass flags through to mob.sh.
Example: mob-claude next -- --stay
Example: mob-claude next -m "my note" -- --stay`,
		Args:                  cobra.ArbitraryArgs,
		FParseErrWhitelist:    cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:                  runNext,
	}
	nextCmd.Flags().SetInterspersed(false)
	nextCmd.Flags().StringVarP(&message, "message", "m", "", "Note for the next driver")
	nextCmd.Flags().BoolVar(&skipSummary, "skip-summary", false, "Skip AI summary generation")

	// Done command
	doneCmd := &cobra.Command{
		Use:   "done [-- mob-flags...]",
		Short: "Complete the mob session",
		Long: `Generates a final summary and runs 'mob done'.

Use -- to pass flags through to mob.sh.
Example: mob-claude done -- --no-squash`,
		Args:                  cobra.ArbitraryArgs,
		FParseErrWhitelist:    cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:                  runDone,
	}
	doneCmd.Flags().SetInterspersed(false)
	doneCmd.Flags().StringVarP(&message, "message", "m", "", "Final note for the session")
	doneCmd.Flags().BoolVar(&skipSummary, "skip-summary", false, "Skip AI summary generation")

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show current session status",
		Long:  "Shows the current plan, recent summaries, and mob status.",
		RunE:  runStatus,
	}

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  "View or update mob-claude configuration.",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	}

	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  "Available keys: apiUrl, teamName, model, maxTurns, skipSummary",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}

	configCmd.AddCommand(configShowCmd, configSetCmd)

	rootCmd.AddCommand(startCmd, nextCmd, doneCmd, statusCmd, configCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	// Handle --help/-h manually since we disabled flag parsing
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return cmd.Help()
		}
	}

	mobWrapper := mob.NewWrapper()

	// Check mob.sh is installed
	if err := mobWrapper.CheckMobInstalled(); err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Warning: could not load config: %v\n", err)
	}

	// Initialize plan manager
	planMgr, err := plans.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plan manager: %w", err)
	}

	// Run mob start (pass all args through to mob.sh)
	fmt.Println("Starting mob session...")
	if err := mobWrapper.Start("", args...); err != nil {
		return fmt.Errorf("mob start failed: %w", err)
	}

	// Get the actual branch we're on now
	currentBranch, err := mobWrapper.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	baseBranch, err := mobWrapper.GetBaseBranch()
	if err != nil {
		baseBranch = currentBranch
	}

	// Get repo URL
	repoURL, err := mobWrapper.GetRepoURL()
	if err != nil {
		repoURL = "unknown"
	}

	// Try to fetch plan from API if configured
	var planText string
	if cfg.TeamName != "" && cfg.APIURL != "" {
		client := api.NewClient(cfg.APIURL, cfg.TeamName)
		remotePlan, err := client.GetPlan(baseBranch)
		if err != nil {
			fmt.Printf("Warning: could not fetch plan from API: %v\n", err)
		} else if remotePlan != "" {
			planText = remotePlan
			fmt.Println("Fetched plan from dashboard")
		}
	}

	// Check local plan
	localPlan, _ := planMgr.LoadPlan(baseBranch)

	// Decide which plan to use
	if planText == "" && localPlan == "" {
		// Create a new plan
		fmt.Printf("Creating new plan for branch: %s\n", baseBranch)
		if err := planMgr.CreateDefaultPlan(baseBranch); err != nil {
			fmt.Printf("Warning: could not create plan: %v\n", err)
		} else {
			fmt.Printf("Plan created at: %s\n", planMgr.GetPlanPath(baseBranch))
		}
	} else if planText != "" && planText != localPlan {
		// Remote plan is newer, save it locally
		if err := planMgr.SavePlan(baseBranch, planText); err != nil {
			fmt.Printf("Warning: could not save plan locally: %v\n", err)
		} else {
			fmt.Println("Synced plan from dashboard")
		}
	} else if localPlan != "" {
		fmt.Printf("Using existing plan: %s\n", planMgr.GetPlanPath(baseBranch))
	}

	// Get current user for driver name
	driverName := getDriverName()

	// Save current session
	session := &config.CurrentSession{
		Branch:     baseBranch,
		RepoURL:    repoURL,
		StartedAt:  time.Now().Format(time.RFC3339),
		DriverName: driverName,
	}

	// Try to register workstream with API
	if cfg.TeamName != "" && cfg.APIURL != "" {
		client := api.NewClient(cfg.APIURL, cfg.TeamName)
		workstream, err := client.CreateWorkstream(repoURL, baseBranch)
		if err != nil {
			fmt.Printf("Warning: could not register with dashboard: %v\n", err)
		} else {
			session.WorkstreamID = workstream.ID
			fmt.Printf("Registered with dashboard: %s/team/%s\n", cfg.APIURL, cfg.TeamName)
		}
	}

	if err := config.SaveCurrentSession(session); err != nil {
		fmt.Printf("Warning: could not save session: %v\n", err)
	}

	fmt.Printf("\nMob session started!\n")
	fmt.Printf("Driver: %s\n", driverName)
	fmt.Printf("Branch: %s\n", currentBranch)

	return nil
}

func runNext(cmd *cobra.Command, args []string) error {
	mobWrapper := mob.NewWrapper()

	if err := mobWrapper.CheckMobInstalled(); err != nil {
		return err
	}

	// Load session
	session, err := config.LoadCurrentSession()
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("no active mob session. Run 'mob-claude start' first")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Initialize managers
	planMgr, err := plans.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plan manager: %w", err)
	}

	// Generate summary unless skipped
	var summaryObj *plans.Summary
	if !skipSummary && !cfg.SkipSummary {
		fmt.Println("Generating rotation summary...")

		diff, err := mobWrapper.GetDiffFromBase()
		if err != nil {
			diff = ""
			fmt.Printf("Warning: could not get diff: %v\n", err)
		}

		gen := summary.NewGenerator(cfg.Model, cfg.MaxTurns)
		summaryObj, err = gen.Generate(diff, message, session.Branch)
		if err != nil {
			fmt.Printf("Warning: summary generation failed: %v\n", err)
		} else {
			summaryObj.DriverName = session.DriverName
			fmt.Printf("Summary: %s\n", summaryObj.TLDR)
		}
	} else if message != "" {
		// Create minimal summary with just the message
		summaryObj = &plans.Summary{
			Timestamp:  time.Now(),
			DriverName: session.DriverName,
			DriverNote: message,
			TLDR:       message,
			Branch:     session.Branch,
		}
	}

	// Save summary locally
	if summaryObj != nil {
		if err := planMgr.SaveSummary(summaryObj); err != nil {
			fmt.Printf("Warning: could not save summary: %v\n", err)
		}
	}

	// Upload to API
	if cfg.TeamName != "" && cfg.APIURL != "" && summaryObj != nil {
		client := api.NewClient(cfg.APIURL, cfg.TeamName)

		// Get current plan for snapshot
		planText, _ := planMgr.LoadPlan(session.Branch)

		startedAt, _ := time.Parse(time.RFC3339, session.StartedAt)

		summaryJSON, _ := json.Marshal(map[string]interface{}{
			"changes":   summaryObj.Changes,
			"nextSteps": summaryObj.NextSteps,
		})

		rotation := &api.CreateRotationRequest{
			DriverName:   session.DriverName,
			DriverNote:   message,
			SummaryTLDR:  summaryObj.TLDR,
			SummaryJSON:  summaryJSON,
			PlanSnapshot: planText,
			StartedAt:    startedAt,
		}

		_, err := client.CreateRotation(session.Branch, rotation)
		if err != nil {
			fmt.Printf("Warning: could not upload rotation: %v\n", err)
		} else {
			fmt.Println("Rotation recorded in dashboard")
		}

		// Sync plan to API
		if planText != "" {
			if err := client.UpdatePlan(session.Branch, planText); err != nil {
				fmt.Printf("Warning: could not sync plan: %v\n", err)
			}
		}
	}

	// Clear session before mob next
	if err := config.ClearCurrentSession(); err != nil {
		fmt.Printf("Warning: could not clear session: %v\n", err)
	}

	// Run mob next
	fmt.Println("\nHanding off to next driver...")
	return mobWrapper.Next(args...)
}

func runDone(cmd *cobra.Command, args []string) error {
	mobWrapper := mob.NewWrapper()

	if err := mobWrapper.CheckMobInstalled(); err != nil {
		return err
	}

	// Load session
	session, err := config.LoadCurrentSession()
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Generate final summary if we have a session
	if session != nil && !skipSummary && !cfg.SkipSummary {
		fmt.Println("Generating final summary...")

		planMgr, err := plans.NewManager()
		if err == nil {
			diff, _ := mobWrapper.GetDiffFromBase()
			gen := summary.NewGenerator(cfg.Model, cfg.MaxTurns)
			summaryObj, err := gen.Generate(diff, message, session.Branch)
			if err == nil {
				summaryObj.DriverName = session.DriverName
				_ = planMgr.SaveSummary(summaryObj)
				fmt.Printf("Final summary: %s\n", summaryObj.TLDR)

				// Upload to API
				if cfg.TeamName != "" && cfg.APIURL != "" {
					client := api.NewClient(cfg.APIURL, cfg.TeamName)
					planText, _ := planMgr.LoadPlan(session.Branch)
					startedAt, _ := time.Parse(time.RFC3339, session.StartedAt)

					summaryJSON, _ := json.Marshal(map[string]interface{}{
						"changes":   summaryObj.Changes,
						"nextSteps": summaryObj.NextSteps,
					})

					rotation := &api.CreateRotationRequest{
						DriverName:   session.DriverName,
						DriverNote:   message,
						SummaryTLDR:  summaryObj.TLDR,
						SummaryJSON:  summaryJSON,
						PlanSnapshot: planText,
						StartedAt:    startedAt,
					}
					_, _ = client.CreateRotation(session.Branch, rotation)
				}
			}
		}
	}

	// Clear session
	_ = config.ClearCurrentSession()

	// Run mob done
	fmt.Println("\nCompleting mob session...")
	return mobWrapper.Done(args...)
}

func runStatus(cmd *cobra.Command, args []string) error {
	mobWrapper := mob.NewWrapper()

	// Show mob status
	fmt.Println("=== Mob Status ===")
	status, err := mobWrapper.Status()
	if err != nil {
		fmt.Printf("mob status: %v\n", err)
	} else {
		fmt.Print(status)
	}

	// Show current session
	session, _ := config.LoadCurrentSession()
	if session != nil {
		fmt.Println("\n=== Current Session ===")
		fmt.Printf("Branch: %s\n", session.Branch)
		fmt.Printf("Driver: %s\n", session.DriverName)
		fmt.Printf("Started: %s\n", session.StartedAt)
	}

	// Show plan
	planMgr, err := plans.NewManager()
	if err == nil {
		branch := ""
		if session != nil {
			branch = session.Branch
		} else {
			branch, _ = mobWrapper.GetBaseBranch()
		}

		if branch != "" {
			plan, err := planMgr.LoadPlan(branch)
			if err == nil && plan != "" {
				fmt.Println("\n=== Plan ===")
				// Show first 20 lines
				lines := splitLines(plan)
				maxLines := 20
				if len(lines) > maxLines {
					for i := 0; i < maxLines; i++ {
						fmt.Println(lines[i])
					}
					fmt.Printf("... (%d more lines)\n", len(lines)-maxLines)
				} else {
					fmt.Print(plan)
				}
			}
		}
	}

	// Show latest summary
	if planMgr != nil {
		latest, _ := planMgr.GetLatestSummary()
		if latest != "" {
			fmt.Println("\n=== Latest Summary ===")
			fmt.Println(latest)
		}
	}

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Current configuration:")
	fmt.Printf("  apiUrl:      %s\n", cfg.APIURL)
	fmt.Printf("  teamName:    %s\n", cfg.TeamName)
	fmt.Printf("  model:       %s\n", cfg.Model)
	fmt.Printf("  maxTurns:    %d\n", cfg.MaxTurns)
	fmt.Printf("  skipSummary: %v\n", cfg.SkipSummary)

	dir, _ := config.GetConfigDir()
	fmt.Printf("\nConfig file: %s/config.json\n", dir)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	switch key {
	case "apiUrl":
		cfg.APIURL = value
	case "teamName":
		cfg.TeamName = value
	case "model":
		cfg.Model = value
	case "maxTurns":
		var turns int
		if _, err := fmt.Sscanf(value, "%d", &turns); err != nil {
			return fmt.Errorf("invalid maxTurns value: %s", value)
		}
		cfg.MaxTurns = turns
	case "skipSummary":
		cfg.SkipSummary = value == "true" || value == "1"
	default:
		return fmt.Errorf("unknown config key: %s\nAvailable keys: apiUrl, teamName, model, maxTurns, skipSummary", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func getDriverName() string {
	// Try git config first
	if name := getGitConfigValue("user.name"); name != "" {
		return name
	}

	// Fall back to system user
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func getGitConfigValue(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
