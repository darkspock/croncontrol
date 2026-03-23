// Package worker provides enrollment logic for CronControl Workers.
//
// Enrollment uses a two-phase flow:
// 1. Admin creates worker → gets temporary enrollment token (stored in DB, expires in 1 hour).
// 2. Worker binary exchanges token for permanent credential (updates DB, clears token).
package worker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/id"
)

// EnrollmentToken is a temporary token used to enroll a worker.
type EnrollmentToken struct {
	WorkerID string `json:"worker_id"`
	Token    string `json:"token"`
	// Token is valid for 1 hour.
	ExpiresAt time.Time `json:"expires_at"`
}

// EnrollmentResult is returned after successful enrollment.
type EnrollmentResult struct {
	WorkerID   string `json:"worker_id"`
	Credential string `json:"credential"` // shown once
}

// Enrollment manages worker registration and credential exchange.
// Tokens are stored in the workers table (enrollment_token_hash, enrollment_token_expires_at)
// to survive process restarts and support multi-instance deployments.
type Enrollment struct {
	queries *db.Queries
}

// NewEnrollment creates a new enrollment manager.
func NewEnrollment(queries *db.Queries) *Enrollment {
	return &Enrollment{
		queries: queries,
	}
}

// CreateWorker creates a new worker record and returns an enrollment token.
// Called by admin via API.
func (e *Enrollment) CreateWorker(ctx context.Context, workspaceID, name string, maxConcurrency int32) (*db.Worker, string, error) {
	// Generate temporary enrollment token
	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	workerID := id.NewWorker()

	// Create worker with a placeholder credential hash (will be replaced on enrollment)
	placeholderHash := hashCredential("placeholder_" + workerID)

	worker, err := e.queries.CreateWorker(ctx, db.CreateWorkerParams{
		ID:             workerID,
		WorkspaceID:    workspaceID,
		Name:           name,
		CredentialHash: placeholderHash,
		MaxConcurrency: maxConcurrency,
	})
	if err != nil {
		return nil, "", fmt.Errorf("create worker: %w", err)
	}

	// Store enrollment token hash in the workers table (expires in 1 hour)
	tokenHash := hashCredential(token)
	expiresAt := time.Now().Add(time.Hour)
	err = e.queries.SetWorkerEnrollmentToken(ctx, db.SetWorkerEnrollmentTokenParams{
		ID:                  workerID,
		EnrollmentTokenHash: &tokenHash,
		EnrollmentTokenExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
	})
	if err != nil {
		return nil, "", fmt.Errorf("store enrollment token: %w", err)
	}

	return &worker, token, nil
}

// Enroll exchanges an enrollment token for a permanent credential.
// Called by the worker binary.
func (e *Enrollment) Enroll(ctx context.Context, token string) (*EnrollmentResult, error) {
	// Look up the worker by the hashed token
	tokenHash := hashCredential(token)
	worker, err := e.queries.GetWorkerByEnrollmentToken(ctx, &tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired enrollment token")
	}

	// Generate permanent credential
	credential, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate credential: %w", err)
	}
	credentialFull := "wrk_cred_" + credential

	// Update worker with real credential hash and clear enrollment token
	credentialHash := hashCredential(credentialFull)
	err = e.queries.UpdateWorkerCredentialHash(ctx, db.UpdateWorkerCredentialHashParams{
		ID:             worker.ID,
		CredentialHash: credentialHash,
	})
	if err != nil {
		return nil, fmt.Errorf("update credential: %w", err)
	}

	return &EnrollmentResult{
		WorkerID:   worker.ID,
		Credential: credentialFull,
	}, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashCredential(credential string) string {
	h := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(h[:])
}
