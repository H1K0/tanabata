package domain

import "context"

type ctxKey int

const userKey ctxKey = iota

type contextUser struct {
	ID      int16
	IsAdmin bool
}

func WithUser(ctx context.Context, userID int16, isAdmin bool) context.Context {
	return context.WithValue(ctx, userKey, contextUser{ID: userID, IsAdmin: isAdmin})
}

func UserFromContext(ctx context.Context) (userID int16, isAdmin bool) {
	u, ok := ctx.Value(userKey).(contextUser)
	if !ok {
		return 0, false
	}
	return u.ID, u.IsAdmin
}
