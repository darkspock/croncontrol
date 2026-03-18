#!/bin/bash
# Seed CronControl with demo data.
# Usage: just seed [url]

set -e

BASE_URL="${1:-http://localhost:8080}"
API_URL="$BASE_URL/api/v1"

echo "Seeding CronControl at $BASE_URL..."

# Register
echo "==> Registering workspace..."
REG=$(curl -sf -X POST "$API_URL/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@croncontrol.dev","name":"Demo Workspace","password":"demodemo1234"}')

API_KEY=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['api_key'])")
echo "    API Key: ${API_KEY:0:25}..."
echo "    Workspace: demo-workspace"

H="-H X-API-Key:$API_KEY -H Content-Type:application/json"

# Create processes
echo "==> Creating processes..."

curl -sf -X POST "$API_URL/processes" $H -d '{
  "name":"daily-report","schedule_type":"cron","schedule":"0 13 * * *",
  "execution_method":"http","method_config":{"url":"https://httpbin.org/post","method":"POST","body":"{\"type\":\"daily_report\"}"},
  "tags":["billing","critical"]
}' > /dev/null
echo "    daily-report (cron 0 13 * * *, HTTP)"

curl -sf -X POST "$API_URL/processes" $H -d '{
  "name":"sync-users","schedule_type":"fixed_delay","delay_duration":"5m",
  "execution_method":"http","method_config":{"url":"https://httpbin.org/post","method":"POST"},
  "tags":["users"]
}' > /dev/null
echo "    sync-users (fixed_delay 5m, HTTP)"

curl -sf -X POST "$API_URL/processes" $H -d '{
  "name":"cleanup-logs","schedule_type":"cron","schedule":"0 3 * * *",
  "execution_method":"http","method_config":{"url":"https://httpbin.org/post","method":"POST"},
  "tags":["maintenance"]
}' > /dev/null
echo "    cleanup-logs (cron 0 3 * * *, HTTP)"

curl -sf -X POST "$API_URL/processes" $H -d '{
  "name":"health-ping","schedule_type":"cron","schedule":"*/1 * * * *",
  "execution_method":"http","method_config":{"url":"https://httpbin.org/get","method":"GET"},
  "tags":["monitoring"]
}' > /dev/null
echo "    health-ping (cron */1 * * * *, HTTP GET)"

curl -sf -X POST "$API_URL/processes" $H -d '{
  "name":"invoice-gen","schedule_type":"on_demand",
  "execution_method":"http","method_config":{"url":"https://httpbin.org/post","method":"POST","body":"{\"action\":\"generate\"}"},
  "tags":["billing"]
}' > /dev/null
echo "    invoice-gen (on_demand, HTTP)"

# Trigger some runs
echo "==> Triggering runs..."
PROCS=$(curl -sf "$API_URL/processes" $H)
for name in daily-report sync-users invoice-gen; do
  PID=$(echo "$PROCS" | python3 -c "import sys,json; [print(p['id']) for p in json.load(sys.stdin)['data'] if p['name']=='$name']" 2>/dev/null)
  if [ -n "$PID" ]; then
    curl -sf -X POST "$API_URL/processes/$PID/trigger" $H > /dev/null
    echo "    Triggered $name"
  fi
done

# Create a queue
echo "==> Creating queue..."
curl -sf -X POST "$API_URL/queues" $H -d '{
  "name":"emails","execution_method":"http",
  "method_config":{"url":"https://httpbin.org/post","method":"POST"},
  "concurrency":3,"max_attempts":3
}' > /dev/null
echo "    emails (HTTP, concurrency=3)"

# Enqueue some jobs
echo "==> Enqueuing jobs..."
QUEUE_ID=$(curl -sf "$API_URL/queues" $H | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['id'])" 2>/dev/null)
for i in 1 2 3 4 5; do
  curl -sf -X POST "$API_URL/jobs" $H -d "{
    \"queue_id\":\"$QUEUE_ID\",
    \"payload\":{\"to\":\"user$i@example.com\",\"subject\":\"Welcome!\"},
    \"reference\":\"user-$i\"
  }" > /dev/null
  echo "    Job $i: user-$i"
done

echo ""
echo "=== Seed complete! ==="
echo "Dashboard: $BASE_URL"
echo "API Key: $API_KEY"
echo ""
echo "To sign in, use:"
echo "  Email: demo@croncontrol.dev"
echo "  Password: demodemo1234"
echo "  Or paste the API key above"
