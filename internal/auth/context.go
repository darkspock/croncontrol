// Package auth provides authentication and authorization for CronControl.
//
// Supports: API key auth (header), Google OAuth (session cookie).
// All authenticated requests are scoped to a workspace, except platform admin requests.
package auth

import (
	"context"
)

type contextKey string

const (
	ctxWorkspaceID    contextKey = "workspace_id"
	ctxUserID         contextKey = "user_id"
	ctxActorType      contextKey = "actor_type"
	ctxActorID        contextKey = "actor_id"
	ctxRole           contextKey = "role"
	ctxPlatformAdmin  contextKey = "platform_admin"
)

// Actor represents who is making the request.
type Actor struct {
	Type            string // "user", "api_key", "worker", "system"
	ID              string // user ID or API key ID
	WorkspaceID     string
	Role            string // "admin", "operator", "viewer"
	IsPlatformAdmin bool   // global platform-level admin
}

// WithActor stores the actor in the context.
func WithActor(ctx context.Context, actor Actor) context.Context {
	ctx = context.WithValue(ctx, ctxWorkspaceID, actor.WorkspaceID)
	ctx = context.WithValue(ctx, ctxUserID, actor.ID)
	ctx = context.WithValue(ctx, ctxActorType, actor.Type)
	ctx = context.WithValue(ctx, ctxActorID, actor.ID)
	ctx = context.WithValue(ctx, ctxRole, actor.Role)
	ctx = context.WithValue(ctx, ctxPlatformAdmin, actor.IsPlatformAdmin)
	return ctx
}

// GetActor extracts the actor from the context.
func GetActor(ctx context.Context) (Actor, bool) {
	wID, _ := ctx.Value(ctxWorkspaceID).(string)
	isPlatformAdmin, _ := ctx.Value(ctxPlatformAdmin).(bool)
	if wID == "" && !isPlatformAdmin {
		return Actor{}, false
	}
	return Actor{
		Type:            ctx.Value(ctxActorType).(string),
		ID:              ctx.Value(ctxActorID).(string),
		WorkspaceID:     wID,
		Role:            ctx.Value(ctxRole).(string),
		IsPlatformAdmin: isPlatformAdmin,
	}, true
}

// WorkspaceID extracts just the workspace ID from context.
func WorkspaceID(ctx context.Context) string {
	wID, _ := ctx.Value(ctxWorkspaceID).(string)
	return wID
}

// Role extracts the role from context.
func Role(ctx context.Context) string {
	r, _ := ctx.Value(ctxRole).(string)
	return r
}

// IsPlatformAdmin checks if the current actor is a platform admin.
func IsPlatformAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(ctxPlatformAdmin).(bool)
	return v
}
