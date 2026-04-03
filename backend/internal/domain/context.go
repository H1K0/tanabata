package domain

import "context"

type ctxKey int

const userKey ctxKey = iota

type contextUser struct {
	ID        int16
	IsAdmin   bool
	SessionID int
}

// WithUser stores user identity and current session ID in ctx.
func WithUser(ctx context.Context, userID int16, isAdmin bool, sessionID int) context.Context {
	return context.WithValue(ctx, userKey, contextUser{
		ID:        userID,
		IsAdmin:   isAdmin,
		SessionID: sessionID,
	})
}

// UserFromContext retrieves user identity from ctx.
// Returns zero values if no user is stored.
func UserFromContext(ctx context.Context) (userID int16, isAdmin bool, sessionID int) {
	u, ok := ctx.Value(userKey).(contextUser)
	if !ok {
		return 0, false, 0
	}
	return u.ID, u.IsAdmin, u.SessionID
}
