// Package workmodel defines owned work identities independently of SQL adapters.
package workmodel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
)

type Effect struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	StoreID    uuid.UUID
	Domain     capability.Domain
	Kind       string
	SubjectID  uuid.UUID
	Key        string
	Payload    json.RawMessage
	Token      uuid.UUID
	Generation int64
	Attempts   int
	LeaseUntil time.Time
}
