package domain

import "time"

// User is an application user.
type User struct {
	ID        int16
	Name      string
	Password  string // bcrypt hash; only populated when needed for auth
	IsAdmin   bool
	CanCreate bool
	IsBlocked bool
}

// Session is an active user session.
type Session struct {
	ID           int
	TokenHash    string
	UserID       int16
	UserAgent    string
	StartedAt    time.Time
	ExpiresAt    *time.Time
	LastActivity time.Time
	IsCurrent    bool // true when this session matches the caller's token
}

// UserPage is an offset-based page of users.
type UserPage struct {
	Items  []User
	Total  int
	Offset int
	Limit  int
}

// SessionList is a list of sessions with a total count.
type SessionList struct {
	Items []Session
	Total int
}
