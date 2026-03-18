# CLI Quickstart

`cronctl` is the command-line tool for CronControl.

## Install

```bash
# Linux
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/cronctl_linux_amd64.tar.gz
tar xzf cronctl_linux_amd64.tar.gz && sudo mv cronctl /usr/local/bin/

# macOS
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/cronctl_darwin_arm64.tar.gz
tar xzf cronctl_darwin_arm64.tar.gz && sudo mv cronctl /usr/local/bin/
```

## Authenticate

```bash
cronctl login --key cc_live_abc123... --url http://localhost:8080
```

Config is stored in `~/.cronctl/config.json`.

## Common Operations

### Processes

```bash
# List all processes
cronctl processes list

# Trigger a manual run
cronctl processes trigger prc_01HYX...

# Pause a process
cronctl processes pause prc_01HYX...

# Resume
cronctl processes resume prc_01HYX...
```

### Runs

```bash
# List recent runs
cronctl runs list

# Filter by state
cronctl runs list --state failed

# Filter by process
cronctl runs list --process prc_01HYX...

# Get run details
cronctl runs get run_01HYX...

# Kill a running execution
cronctl runs kill run_01HYX...
```

### Queues and Jobs

```bash
# List queues
cronctl queues list

# Create a queue
cronctl queues create --name emails --method http --url https://api.example.com/send

# Enqueue a job
cronctl jobs enqueue --queue que_01HYX... --payload '{"to":"user@example.com"}'

# List failed jobs
cronctl jobs list --state failed

# Get job with attempt history
cronctl jobs get job_01HYX...
```

### Workers

```bash
# List workers
cronctl workers list

# Enroll a new worker
cronctl workers enroll --name prod-worker --url http://localhost:8080
```

### Health

```bash
cronctl health
cronctl version
```

## Output Formats

All commands output JSON by default, suitable for piping to `jq`:

```bash
cronctl runs list --state failed | jq '.data[].id'
```
