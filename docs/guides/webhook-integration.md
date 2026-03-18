# Webhook Integration Guide

CronControl delivers event notifications to your endpoints via HMAC-signed webhooks.

## 1. Create Subscription

```bash
curl -X POST https://your-instance.com/api/v1/webhook-subscriptions \
  -H "X-API-Key: cc_live_..." \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhooks/croncontrol",
    "secret": "your-hmac-secret-min-32-chars-recommended",
    "event_types": ["run.*", "job.failed"]
  }'
```

## 2. Receive Events

CronControl sends a POST request with:

```http
POST /webhooks/croncontrol HTTP/1.1
Content-Type: application/json
X-CronControl-Signature: <hmac-sha256-hex>
X-CronControl-Timestamp: 2026-03-18T15:30:00Z
X-CronControl-Delivery-Id: dlv_01HYX...
```

```json
{
  "id": "evt_01HYX...",
  "type": "run.failed",
  "timestamp": "2026-03-18T15:30:00Z",
  "workspace": { "id": "wsp_01HYX..." },
  "data": {
    "run_id": "run_01HYX...",
    "process_id": "prc_01HYX...",
    "process_name": "daily-report",
    "state": "failed",
    "exit_code": 1,
    "attempt": 3,
    "duration_ms": 5200
  }
}
```

## 3. Verify Signature

Always verify the HMAC-SHA256 signature before processing:

### Node.js
```javascript
const crypto = require('crypto');

function verifySignature(body, secret, signature) {
  const expected = crypto.createHmac('sha256', secret).update(body).digest('hex');
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(signature));
}
```

### Python
```python
import hmac, hashlib

def verify_signature(body: bytes, secret: str, signature: str) -> bool:
    expected = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

### Go
```go
func verifySignature(body []byte, secret, signature string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### PHP
```php
function verifySignature(string $body, string $secret, string $signature): bool {
    $expected = hash_hmac('sha256', $body, $secret);
    return hash_equals($expected, $signature);
}
```

## 4. Event Types

| Event | Description |
|-------|-------------|
| `run.completed` | A run finished successfully |
| `run.failed` | A run failed after all retries |
| `run.hung` | A run exceeded its timeout |
| `run.killed` | A run was manually killed |
| `job.completed` | A job finished successfully |
| `job.failed` | A job failed after all retries |
| `job.killed` | A job was manually killed |
| `usage.warning` | Workspace usage at 80% |
| `webhook.disabled` | This subscription auto-disabled (20 consecutive failures) |
| `worker.offline` | A worker went offline |
| `worker.unhealthy` | A worker is unhealthy |

### Wildcard Filters

- `run.*` matches all run events
- `job.*` matches all job events
- Empty `event_types` matches all events

## 5. Test Delivery

Test your endpoint without a real event:

```bash
curl -X POST https://your-instance.com/api/v1/webhook-subscriptions/whs_01HYX.../test \
  -H "X-API-Key: cc_live_..."
```

## 6. Delivery Guarantees

- **At-least-once**: events may be delivered more than once. Use `X-CronControl-Delivery-Id` to deduplicate.
- **3 retries**: failed deliveries retry with 5s, 10s backoff.
- **Auto-disable**: after 20 consecutive failures, the subscription is disabled. Re-enable manually via dashboard or API.
- **Respond with 2xx**: any 2xx response is treated as success. Other status codes trigger a retry.
