-- name: CreateWebhookSubscription :one
INSERT INTO webhook_subscriptions (id, workspace_id, url, secret, event_types)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWebhookSubscription :one
SELECT * FROM webhook_subscriptions WHERE id = $1 AND workspace_id = $2;

-- name: ListWebhookSubscriptionsByWorkspace :many
SELECT * FROM webhook_subscriptions WHERE workspace_id = $1 ORDER BY created_at;

-- name: UpdateWebhookSubscription :one
UPDATE webhook_subscriptions SET
    url = $3,
    event_types = $4,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteWebhookSubscription :execrows
DELETE FROM webhook_subscriptions WHERE id = $1 AND workspace_id = $2;

-- name: SetWebhookEnabled :exec
UPDATE webhook_subscriptions SET enabled = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: IncrementWebhookFailures :exec
UPDATE webhook_subscriptions SET
    consecutive_failures = consecutive_failures + 1,
    last_failure_at = now(),
    auto_disabled = CASE WHEN consecutive_failures + 1 >= 20 THEN true ELSE false END,
    enabled = CASE WHEN consecutive_failures + 1 >= 20 THEN false ELSE enabled END,
    updated_at = now()
WHERE id = $1;

-- name: ResetWebhookFailures :exec
UPDATE webhook_subscriptions SET
    consecutive_failures = 0,
    last_delivery_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: ListActiveWebhooksForEvent :many
SELECT * FROM webhook_subscriptions
WHERE workspace_id = $1 AND enabled = true AND auto_disabled = false;
