package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// AuditService records user actions to the audit trail.
type AuditService struct {
	repo port.AuditRepo
}

func NewAuditService(repo port.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// Log records an action performed by the user in ctx.
// objectType and objectID are optional — pass nil when the action has no target object.
// details can be any JSON-serializable value, or nil.
func (s *AuditService) Log(
	ctx context.Context,
	action string,
	objectType *string,
	objectID *uuid.UUID,
	details any,
) error {
	userID, _, _ := domain.UserFromContext(ctx)

	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("AuditService.Log marshal details: %w", err)
		}
		raw = b
	}

	entry := domain.AuditEntry{
		UserID:     userID,
		Action:     action,
		ObjectType: objectType,
		ObjectID:   objectID,
		Details:    raw,
	}
	return s.repo.Log(ctx, entry)
}

// Query returns a filtered, paginated page of audit log entries.
func (s *AuditService) Query(ctx context.Context, filter domain.AuditFilter) (*domain.AuditPage, error) {
	return s.repo.List(ctx, filter)
}
