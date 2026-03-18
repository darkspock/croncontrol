// Package id provides prefixed ULID generation for all CronControl resources.
//
// All canonical resource IDs use prefix + ULID format (e.g., "wsp_01HYX...").
// Prefixes are defined in the product specification.
package id

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
)

// Resource prefixes — canonical set from docs/product-specification.md
const (
	PrefixWorkspace   = "wsp_"
	PrefixUser        = "usr_"
	PrefixMembership  = "wmb_"
	PrefixWorker      = "wrk_"
	PrefixProcess     = "prc_"
	PrefixRun         = "run_"
	PrefixRunAttempt  = "rat_"
	PrefixQueue       = "que_"
	PrefixJob         = "job_"
	PrefixJobAttempt  = "jat_"
	PrefixWebhookSub  = "whs_"
	PrefixAPIKey      = "key_"
	PrefixSSHCred     = "ssh_"
	PrefixSSMProfile  = "ssp_"
	PrefixK8sCluster  = "k8c_"
)

// validPrefixes for validation.
var validPrefixes = map[string]bool{
	PrefixWorkspace:  true,
	PrefixUser:       true,
	PrefixMembership: true,
	PrefixWorker:     true,
	PrefixProcess:    true,
	PrefixRun:        true,
	PrefixRunAttempt: true,
	PrefixQueue:      true,
	PrefixJob:        true,
	PrefixJobAttempt: true,
	PrefixWebhookSub: true,
	PrefixAPIKey:     true,
	PrefixSSHCred:    true,
	PrefixSSMProfile: true,
	PrefixK8sCluster: true,
}

// New generates a new prefixed ULID.
func New(prefix string) string {
	return prefix + ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// Parse splits a prefixed ID into its prefix and ULID parts.
// Returns an error if the format is invalid.
func Parse(id string) (prefix string, raw string, err error) {
	idx := strings.Index(id, "_")
	if idx == -1 || idx == len(id)-1 {
		return "", "", fmt.Errorf("invalid id format: missing prefix separator in %q", id)
	}

	prefix = id[:idx+1]
	raw = id[idx+1:]

	if !validPrefixes[prefix] {
		return "", "", fmt.Errorf("unknown prefix %q in id %q", prefix, id)
	}

	if _, err := ulid.Parse(raw); err != nil {
		return "", "", fmt.Errorf("invalid ULID in id %q: %w", id, err)
	}

	return prefix, raw, nil
}

// IsValid checks if an ID has valid format without returning parsed components.
func IsValid(id string) bool {
	_, _, err := Parse(id)
	return err == nil
}

// HasPrefix checks if an ID has the expected prefix.
func HasPrefix(id, expectedPrefix string) bool {
	return strings.HasPrefix(id, expectedPrefix)
}

// Convenience generators for each resource type.
func NewWorkspace() string   { return New(PrefixWorkspace) }
func NewUser() string        { return New(PrefixUser) }
func NewMembership() string  { return New(PrefixMembership) }
func NewWorker() string      { return New(PrefixWorker) }
func NewProcess() string     { return New(PrefixProcess) }
func NewRun() string         { return New(PrefixRun) }
func NewRunAttempt() string  { return New(PrefixRunAttempt) }
func NewQueue() string       { return New(PrefixQueue) }
func NewJob() string         { return New(PrefixJob) }
func NewJobAttempt() string  { return New(PrefixJobAttempt) }
func NewWebhookSub() string  { return New(PrefixWebhookSub) }
func NewAPIKey() string      { return New(PrefixAPIKey) }
func NewSSHCred() string     { return New(PrefixSSHCred) }
func NewSSMProfile() string  { return New(PrefixSSMProfile) }
func NewK8sCluster() string  { return New(PrefixK8sCluster) }
