// Package notifier delivers webhook events to subscriber endpoints.
//
// Canonical rules (docs/product-specification.md):
// - Multiple subscriptions per workspace.
// - HMAC-SHA256 signatures.
// - At-least-once delivery.
// - Auto-disable after 20 consecutive failures.
// - Reactivation is manual only.
package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/id"
)

// Event represents a webhook event payload.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Workspace struct {
		ID string `json:"id"`
	} `json:"workspace"`
	Data any `json:"data"`
}

// Notifier dispatches events to webhook subscribers.
type Notifier struct {
	queries *db.Queries
	client  *http.Client
}

// New creates a new webhook Notifier.
func New(queries *db.Queries) *Notifier {
	return &Notifier{
		queries: queries,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Emit dispatches an event to all matching subscribers for a workspace.
// Non-blocking: runs in a goroutine per subscription.
func (n *Notifier) Emit(ctx context.Context, workspaceID string, event Event) {
	event.ID = id.New("evt_")
	event.Timestamp = time.Now().UTC()
	event.Workspace.ID = workspaceID

	subs, err := n.queries.ListActiveWebhooksForEvent(ctx, workspaceID)
	if err != nil {
		slog.Error("notifier: list subscriptions", "error", err)
		return
	}

	for _, sub := range subs {
		if !matchesFilter(event.Type, sub.EventTypes) {
			continue
		}
		go n.deliver(context.Background(), sub, event)
	}
}

// deliver sends the event to a single subscription with retry.
func (n *Notifier) deliver(ctx context.Context, sub db.WebhookSubscription, event Event) {
	body, err := json.Marshal(event)
	if err != nil {
		slog.Error("notifier: marshal event", "error", err)
		return
	}

	deliveryID := id.New("dlv_")
	signature := sign(body, sub.Secret)

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", sub.Url, bytes.NewReader(body))
		if err != nil {
			slog.Error("notifier: create request", "error", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CronControl-Signature", signature)
		req.Header.Set("X-CronControl-Timestamp", event.Timestamp.Format(time.RFC3339))
		req.Header.Set("X-CronControl-Delivery-Id", deliveryID)

		resp, err := n.client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			// Success — reset failure counter
			n.queries.ResetWebhookFailures(ctx, sub.ID)
			slog.Debug("notifier: delivered", "subscription", sub.ID, "event", event.Type)
			return
		}

		if resp != nil {
			resp.Body.Close()
		}

		slog.Warn("notifier: delivery failed",
			"subscription", sub.ID,
			"attempt", attempt,
			"error", err,
		)

		if attempt < 3 {
			time.Sleep(time.Duration(attempt*5) * time.Second) // 5s, 10s backoff
		}
	}

	// All attempts failed
	n.queries.IncrementWebhookFailures(ctx, sub.ID)
	slog.Error("notifier: all delivery attempts failed",
		"subscription", sub.ID,
		"event", event.Type,
	)
}

// matchesFilter checks if an event type matches the subscription's filter.
// Supports exact match and wildcard (e.g., "run.*" matches "run.completed").
func matchesFilter(eventType string, filters []string) bool {
	if len(filters) == 0 {
		return true // no filter = match all
	}
	for _, f := range filters {
		if f == eventType {
			return true
		}
		if strings.HasSuffix(f, ".*") {
			prefix := strings.TrimSuffix(f, ".*")
			if strings.HasPrefix(eventType, prefix+".") {
				return true
			}
		}
	}
	return false
}

// sign creates an HMAC-SHA256 signature of the payload.
func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
