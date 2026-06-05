// Package audit holds the audit-log entry entity used to record who changed
// what, for every mutating action across the platform.
package audit

import (
	"encoding/json"
	"time"
)

// Entry is a single audit-log record.
type Entry struct {
	ID          int64
	TenantID    int64
	ActorUserID *int64 // nil for system actions
	Action      string // e.g. "user.create"
	Entity      string // e.g. "user"
	EntityID    string // stringified id of the affected entity
	Diff        json.RawMessage
	IP          string
	CreatedAt   time.Time
}
