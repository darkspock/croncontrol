"""Example usage of the CronControl Python SDK."""

from croncontrol import CronControl

# Initialize client (reads CRONCONTROL_URL and CRONCONTROL_API_KEY from env)
cc = CronControl()

# Or with explicit config:
# cc = CronControl("http://localhost:8080", "cc_live_abc123...")

# List processes
result = cc.list_processes()
for proc in result["data"]:
    print(f"  {proc['name']} ({proc['schedule_type']})")

# Trigger a process
run = cc.trigger_process("prc_01HYX...")
print(f"Run started: {run['data']['id']}")

# Report heartbeat progress (inside a running job)
for i in range(100):
    # ... do work ...
    cc.heartbeat("run_01HYX...", total=100, current=i + 1, message=f"Step {i + 1}")

# Enqueue a job
job = cc.enqueue_job(
    queue_id="que_01HYX...",
    payload={"to": "user@example.com", "subject": "Hello"},
    reference="order-12345",
)
print(f"Job enqueued: {job['data']['id']}")

# Check health
health = cc.health()
print(f"Status: {health['status']}")
