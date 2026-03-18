# MCP Server Setup Guide

Use CronControl from AI agents via the Model Context Protocol (MCP).

## Prerequisites

- CronControl server running
- API key with operator or admin role
- Claude Desktop, Claude Code, or any MCP-compatible client

## 1. Download MCP Binary

```bash
# Linux
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/croncontrol-mcp_linux_amd64.tar.gz
tar xzf croncontrol-mcp_linux_amd64.tar.gz

# macOS
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/croncontrol-mcp_darwin_arm64.tar.gz
tar xzf croncontrol-mcp_darwin_arm64.tar.gz
```

## 2. Configure Environment

```bash
export CRONCONTROL_URL=http://localhost:8080
export CRONCONTROL_API_KEY=cc_live_...
```

## 3. Configure Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "croncontrol": {
      "command": "/usr/local/bin/croncontrol-mcp",
      "env": {
        "CRONCONTROL_URL": "http://localhost:8080",
        "CRONCONTROL_API_KEY": "cc_live_..."
      }
    }
  }
}
```

## 4. Configure Claude Code

Add to `.claude/settings.json`:

```json
{
  "mcpServers": {
    "croncontrol": {
      "command": "croncontrol-mcp",
      "env": {
        "CRONCONTROL_URL": "http://localhost:8080",
        "CRONCONTROL_API_KEY": "cc_live_..."
      }
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `list_processes` | List all processes |
| `create_process` | Create a new process |
| `trigger_process` | Trigger a manual run |
| `pause_process` | Pause a process |
| `resume_process` | Resume a paused process |
| `list_runs` | List runs with filters |
| `get_run` | Get run details |
| `kill_run` | Kill a running execution |
| `list_jobs` | List jobs |
| `enqueue_job` | Enqueue a new job |
| `get_job` | Get job with attempt history |
| `replay_job` | Replay a failed job |
| `get_health` | Check system health |

## Example Prompts

- "List all failed runs from today"
- "Trigger the daily-report process"
- "Show me the last 5 runs for the sync-users process"
- "Enqueue a job to the emails queue with payload {to: 'user@example.com'}"
- "Kill run run_01HYX..."
