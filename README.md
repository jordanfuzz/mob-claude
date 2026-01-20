# mob-claude

A CLI tool that wraps [mob.sh](https://mob.sh) with Claude Code integration for enhanced mob programming sessions.

## Features

- **Plan Management**: Automatically creates and syncs plan files with your mob sessions
- **AI-Powered Summaries**: Generates rotation summaries using Claude to help the next driver
- **Dashboard Integration**: Syncs with the mob-claude-dashboard for real-time visibility
- **Git-Native**: Plans travel with your branch via git

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/mob-claude/mob-claude.git
cd mob-claude

# Build
go build -o mob-claude ./cmd/mob-claude

# Install to PATH
sudo mv mob-claude /usr/local/bin/
```

### Prerequisites

- [mob.sh](https://mob.sh) installed and configured
- [Claude CLI](https://claude.ai/code) (optional, for AI summaries)
- Git

## Quick Start

```bash
# Configure your team name (for dashboard integration)
mob-claude config set teamName my-team
mob-claude config set apiUrl http://localhost:3000

# Start a mob session
mob-claude start feature-auth

# When you're done with your turn
mob-claude next --message "Added login form validation"

# Complete the session
mob-claude done
```

## Commands

### `mob-claude start [branch]`

Starts or joins a mob session. This:
- Runs `mob start`
- Creates/fetches the plan file for the branch
- Registers the workstream with the dashboard (if configured)

```bash
mob-claude start feature-auth
```

### `mob-claude next [--message "..."]`

Hands off to the next driver. This:
- Generates an AI summary of your changes (unless `--skip-summary`)
- Uploads the rotation to the dashboard
- Runs `mob next`

```bash
mob-claude next --message "Implemented OAuth flow"
mob-claude next --skip-summary  # Skip AI summary
```

### `mob-claude done [--message "..."]`

Completes the mob session. This:
- Generates a final summary
- Runs `mob done` (squash commits)

```bash
mob-claude done --message "Feature complete"
```

### `mob-claude status`

Shows the current session status, plan, and recent summaries.

```bash
mob-claude status
```

### `mob-claude config`

View or update configuration.

```bash
# Show current config
mob-claude config show

# Set values
mob-claude config set apiUrl http://localhost:3000
mob-claude config set teamName my-team
mob-claude config set model haiku  # AI model for summaries
mob-claude config set skipSummary true  # Disable AI summaries
```

## Configuration

Configuration is stored in `.claude/mob/config.json` in your project directory.

| Key | Description | Default |
|-----|-------------|---------|
| `apiUrl` | Dashboard API URL | `http://localhost:3000` |
| `teamName` | Your team name for the dashboard | (none) |
| `model` | Claude model for summaries | `haiku` |
| `maxTurns` | Max turns for summary generation | `3` |
| `skipSummary` | Disable AI summaries | `false` |

## File Structure

mob-claude creates the following files in your project:

```
your-project/
├── .claude/
│   ├── plans/
│   │   └── mob-{branch}.md    # Plan file for each branch
│   └── mob/
│       ├── config.json        # mob-claude configuration
│       ├── current.json       # Current session metadata
│       └── summaries/         # Local summary backups
│           └── {timestamp}.json
```

## Dashboard Integration

mob-claude integrates with [mob-claude-dashboard](../mob-claude-dashboard) for real-time visibility into mob sessions.

When configured with a team name and API URL:
- Workstreams are automatically registered
- Plans are synced on rotation
- Rotations and summaries are uploaded

## Development

```bash
# Run tests
go test ./...

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o mob-claude-linux ./cmd/mob-claude
GOOS=darwin GOARCH=amd64 go build -o mob-claude-darwin ./cmd/mob-claude
GOOS=windows GOARCH=amd64 go build -o mob-claude.exe ./cmd/mob-claude
```

## License

MIT
