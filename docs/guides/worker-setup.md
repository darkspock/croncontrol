# Worker Setup Guide

Deploy a CronControl Worker to execute tasks inside your private network.

## Prerequisites

- CronControl server running and accessible
- Admin API key for your workspace
- A server or VM in your private network

## 1. Create Worker

```bash
curl -X POST https://your-instance.com/api/v1/workers \
  -H "X-API-Key: cc_live_..." \
  -H "Content-Type: application/json" \
  -d '{"name": "prod-worker-01", "max_concurrency": 5}'
```

Save the `enrollment_token` from the response.

## 2. Download Worker Binary

```bash
# Linux (amd64)
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/croncontrol-worker_linux_amd64.tar.gz
tar xzf croncontrol-worker_linux_amd64.tar.gz

# macOS (arm64)
curl -LO https://github.com/darkspock/croncontrol/releases/latest/download/croncontrol-worker_darwin_arm64.tar.gz
tar xzf croncontrol-worker_darwin_arm64.tar.gz
```

## 3. Enroll Worker

```bash
./croncontrol-worker --url https://your-instance.com --credential enroll_cc_live_...
```

The worker exchanges the enrollment token for a permanent credential and starts polling for tasks.

## 4. Run as Service (systemd)

```ini
[Unit]
Description=CronControl Worker
After=network.target

[Service]
ExecStart=/usr/local/bin/croncontrol-worker
Environment=CRONCONTROL_URL=https://your-instance.com
Environment=CRONCONTROL_CREDENTIAL=wrk_cred_...
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable croncontrol-worker
sudo systemctl start croncontrol-worker
```

## 5. Configure Process for Worker

Set `runtime: worker` on any process to route execution through the worker:

```bash
curl -X POST https://your-instance.com/api/v1/processes \
  -H "X-API-Key: cc_live_..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internal-backup",
    "schedule_type": "cron",
    "schedule": "0 2 * * *",
    "execution_method": "ssh",
    "runtime": "worker",
    "method_config": {
      "host": "10.0.1.50",
      "command": "/opt/scripts/backup.sh",
      "ssh_credential_id": "ssh_01HYX..."
    }
  }'
```

## Verification

Check worker status in the dashboard under Settings > Workers, or via API:

```bash
curl https://your-instance.com/api/v1/workers -H "X-API-Key: cc_live_..."
```

The worker should show `status: "online"` within 60 seconds.
