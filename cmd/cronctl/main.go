// cronctl is the CronControl CLI tool.
//
// Usage:
//   cronctl login --key cc_live_...
//   cronctl processes list
//   cronctl processes trigger <id>
//   cronctl runs list --state failed
//   cronctl jobs enqueue --queue <id> --payload '{"key":"value"}'
//   cronctl health
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/croncontrol/croncontrol/mcp"
)

var version = "dev"

type Config struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "login":
		handleLogin()
	case "version":
		fmt.Printf("cronctl %s\n", version)
	case "health":
		withClient(func(c *mcp.APIClient) {
			result, err := c.GetRaw(nil, "/workspace/health")
			exitOnErr(err)
			printJSON(result)
		})
	case "processes":
		handleProcesses()
	case "runs":
		handleRuns()
	case "queues":
		handleQueues()
	case "jobs":
		handleJobs()
	case "workers":
		handleWorkers()
	case "admin":
		handleAdmin()
	case "completion":
		handleCompletion()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func handleLogin() {
	key := getFlag("--key", "")
	url := getFlag("--url", "http://localhost:8080")

	if key == "" {
		fmt.Fprint(os.Stderr, "API Key: ")
		fmt.Scanln(&key)
	}

	cfg := Config{URL: url, APIKey: key}
	saveConfig(cfg)
	fmt.Println("Logged in successfully.")
}

func handleProcesses() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cronctl processes <list|trigger|pause|resume> [args]")
		os.Exit(1)
	}

	action := os.Args[2]
	withClient(func(c *mcp.APIClient) {
		switch action {
		case "list":
			result, err := c.Get(nil, "/processes")
			exitOnErr(err)
			printJSON(result)
		case "trigger":
			requireArg(3, "process ID")
			result, err := c.Post(nil, fmt.Sprintf("/processes/%s/trigger", os.Args[3]), nil)
			exitOnErr(err)
			printJSON(result)
		case "pause":
			requireArg(3, "process ID")
			_, err := c.Post(nil, fmt.Sprintf("/processes/%s/pause", os.Args[3]), nil)
			exitOnErr(err)
			fmt.Println("Process paused.")
		case "resume":
			requireArg(3, "process ID")
			_, err := c.Post(nil, fmt.Sprintf("/processes/%s/resume", os.Args[3]), nil)
			exitOnErr(err)
			fmt.Println("Process resumed.")
		default:
			fmt.Fprintf(os.Stderr, "Unknown action: processes %s\n", action)
			os.Exit(1)
		}
	})
}

func handleRuns() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cronctl runs <list|get|kill> [args]")
		os.Exit(1)
	}

	action := os.Args[2]
	withClient(func(c *mcp.APIClient) {
		switch action {
		case "list":
			query := ""
			if state := getFlag("--state", ""); state != "" {
				query = "?state=" + state
			}
			result, err := c.Get(nil, "/runs"+query)
			exitOnErr(err)
			printJSON(result)
		case "get":
			requireArg(3, "run ID")
			result, err := c.Get(nil, fmt.Sprintf("/runs/%s", os.Args[3]))
			exitOnErr(err)
			printJSON(result)
		case "kill":
			requireArg(3, "run ID")
			_, err := c.Post(nil, fmt.Sprintf("/runs/%s/kill", os.Args[3]), nil)
			exitOnErr(err)
			fmt.Println("Kill requested.")
		default:
			fmt.Fprintf(os.Stderr, "Unknown action: runs %s\n", action)
			os.Exit(1)
		}
	})
}

func handleQueues() {
	withClient(func(c *mcp.APIClient) {
		result, err := c.Get(nil, "/queues")
		exitOnErr(err)
		printJSON(result)
	})
}

func handleJobs() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cronctl jobs <list|get|enqueue|replay> [args]")
		os.Exit(1)
	}

	action := os.Args[2]
	withClient(func(c *mcp.APIClient) {
		switch action {
		case "list":
			query := ""
			if state := getFlag("--state", ""); state != "" {
				query = "?state=" + state
			}
			result, err := c.Get(nil, "/jobs"+query)
			exitOnErr(err)
			printJSON(result)
		case "get":
			requireArg(3, "job ID")
			result, err := c.Get(nil, fmt.Sprintf("/jobs/%s", os.Args[3]))
			exitOnErr(err)
			printJSON(result)
		case "enqueue":
			queueID := getFlag("--queue", "")
			payload := getFlag("--payload", "{}")
			if queueID == "" {
				fmt.Fprintln(os.Stderr, "Missing --queue")
				os.Exit(1)
			}
			var p any
			json.Unmarshal([]byte(payload), &p)
			result, err := c.Post(nil, "/jobs", map[string]any{
				"queue_id": queueID,
				"payload":  p,
				"reference": getFlag("--reference", ""),
			})
			exitOnErr(err)
			printJSON(result)
		case "replay":
			requireArg(3, "job ID")
			result, err := c.Post(nil, fmt.Sprintf("/jobs/%s/replay", os.Args[3]), nil)
			exitOnErr(err)
			printJSON(result)
		default:
			fmt.Fprintf(os.Stderr, "Unknown action: jobs %s\n", action)
			os.Exit(1)
		}
	})
}

// Helpers

func withClient(fn func(*mcp.APIClient)) {
	cfg := loadConfig()
	client := mcp.NewAPIClient(cfg.URL, cfg.APIKey)
	fn(client)
}

func loadConfig() Config {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Not logged in. Run: cronctl login --key <your-api-key>")
		os.Exit(1)
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) {
	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0600)
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cronctl", "config.json")
}

func getFlag(name, defaultVal string) string {
	for i, arg := range os.Args {
		if arg == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"=")
		}
	}
	return defaultVal
}

func requireArg(idx int, name string) {
	if len(os.Args) <= idx {
		fmt.Fprintf(os.Stderr, "Missing required argument: %s\n", name)
		os.Exit(1)
	}
}

func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func handleAdmin() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cronctl admin <stats|workspaces|users|promote|impersonate> [args]")
		os.Exit(1)
	}
	action := os.Args[2]
	withClient(func(c *mcp.APIClient) {
		switch action {
		case "stats":
			result, err := c.Get(nil, "/admin/stats")
			exitOnErr(err)
			printJSON(result)
		case "workspaces":
			result, err := c.Get(nil, "/admin/workspaces")
			exitOnErr(err)
			printJSON(result)
		case "users":
			result, err := c.Get(nil, "/admin/users")
			exitOnErr(err)
			printJSON(result)
		case "promote":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: cronctl admin promote <user_id>")
				os.Exit(1)
			}
			userID := os.Args[3]
			result, err := c.Post(nil, "/admin/users/"+userID+"/platform-admin", map[string]any{"is_platform_admin": true})
			exitOnErr(err)
			printJSON(result)
			fmt.Println("User promoted to platform admin.")
		case "demote":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: cronctl admin demote <user_id>")
				os.Exit(1)
			}
			userID := os.Args[3]
			result, err := c.Post(nil, "/admin/users/"+userID+"/platform-admin", map[string]any{"is_platform_admin": false})
			exitOnErr(err)
			printJSON(result)
			fmt.Println("Platform admin revoked.")
		case "impersonate":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: cronctl admin impersonate <workspace_id>")
				os.Exit(1)
			}
			wsID := os.Args[3]
			result, err := c.Post(nil, "/admin/workspaces/"+wsID+"/impersonate", nil)
			exitOnErr(err)
			printJSON(result)
		default:
			fmt.Fprintf(os.Stderr, "Unknown admin action: %s\n", action)
		}
	})
}

func handleWorkers() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cronctl workers <list|enroll> [args]")
		os.Exit(1)
	}
	action := os.Args[2]
	withClient(func(c *mcp.APIClient) {
		switch action {
		case "list":
			result, err := c.Get(nil, "/workers")
			exitOnErr(err)
			printJSON(result)
		case "enroll":
			name := getFlag("--name", "worker")
			result, err := c.Post(nil, "/workers", map[string]any{"name": name})
			exitOnErr(err)
			printJSON(result)
		default:
			fmt.Fprintf(os.Stderr, "Unknown workers action: %s\n", action)
		}
	})
}

func handleCompletion() {
	shell := "bash"
	if len(os.Args) > 2 {
		shell = os.Args[2]
	}

	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s (supported: bash, zsh)\n", shell)
		os.Exit(1)
	}
}

const bashCompletion = `# cronctl bash completion
# Add to ~/.bashrc: eval "$(cronctl completion bash)"
_cronctl_completions() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local prev="${COMP_WORDS[COMP_CWORD-1]}"

  case "${COMP_WORDS[1]}" in
    processes) COMPREPLY=($(compgen -W "list trigger pause resume" -- "$cur")) ;;
    runs) COMPREPLY=($(compgen -W "list get kill" -- "$cur")) ;;
    queues) COMPREPLY=($(compgen -W "list create" -- "$cur")) ;;
    jobs) COMPREPLY=($(compgen -W "list get enqueue replay cancel" -- "$cur")) ;;
    workers) COMPREPLY=($(compgen -W "list enroll" -- "$cur")) ;;
    *) COMPREPLY=($(compgen -W "login processes runs queues jobs workers health version completion help" -- "$cur")) ;;
  esac
}
complete -F _cronctl_completions cronctl
`

const zshCompletion = `# cronctl zsh completion
# Add to ~/.zshrc: eval "$(cronctl completion zsh)"
_cronctl() {
  local -a commands subcommands
  commands=(
    'login:Authenticate with an API key'
    'processes:Manage processes'
    'runs:View and manage runs'
    'queues:Manage queues'
    'jobs:Manage jobs'
    'workers:Manage workers'
    'health:Check system health'
    'version:Show version'
    'completion:Generate shell completions'
    'help:Show help'
  )

  _arguments -C '1:command:->cmd' '*::arg:->args'

  case $state in
    cmd) _describe 'commands' commands ;;
    args)
      case ${words[1]} in
        processes) subcommands=('list' 'trigger' 'pause' 'resume'); _describe 'subcommands' subcommands ;;
        runs) subcommands=('list' 'get' 'kill'); _describe 'subcommands' subcommands ;;
        queues) subcommands=('list' 'create'); _describe 'subcommands' subcommands ;;
        jobs) subcommands=('list' 'get' 'enqueue' 'replay' 'cancel'); _describe 'subcommands' subcommands ;;
        workers) subcommands=('list' 'enroll'); _describe 'subcommands' subcommands ;;
        completion) subcommands=('bash' 'zsh'); _describe 'shells' subcommands ;;
      esac ;;
  esac
}
compdef _cronctl cronctl
`

func printUsage() {
	fmt.Println(`cronctl - CronControl CLI

Usage:
  cronctl login --key <api-key> [--url <url>]
  cronctl processes list|trigger|pause|resume <id>
  cronctl runs list [--state <state>] | get|kill <id>
  cronctl queues list|create --name <name> --method <method>
  cronctl jobs list [--state <state>] | get|enqueue|replay|cancel
  cronctl workers list|enroll --name <name>
  cronctl admin stats|workspaces|users|promote|demote|impersonate
  cronctl health
  cronctl version
  cronctl completion bash|zsh`)
}
