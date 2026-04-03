package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// Claims is the JWT payload for both access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	UserID    int16 `json:"uid"`
	IsAdmin   bool  `json:"adm"`
	SessionID int   `json:"sid"`
}

// TokenPair holds an issued access/refresh token pair with the access TTL.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // access token TTL in seconds
}

// AuthService handles authentication and session lifecycle.
type AuthService struct {
	users    port.UserRepo
	sessions port.SessionRepo
	secret   []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService creates an AuthService.
func NewAuthService(
	users port.UserRepo,
	sessions port.SessionRepo,
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		sessions:   sessions,
		secret:     []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Login validates credentials, creates a session, and returns a token pair.
func (s *AuthService) Login(ctx context.Context, name, password, userAgent string) (*TokenPair, error) {
	user, err := s.users.GetByName(ctx, name)
	if err != nil {
		// Return ErrUnauthorized regardless of whether the user exists,
		// to avoid username enumeration.
		return nil, domain.ErrUnauthorized
	}

	if user.IsBlocked {
		return nil, domain.ErrForbidden
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	var expiresAt *time.Time
	if s.refreshTTL > 0 {
		t := time.Now().Add(s.refreshTTL)
		expiresAt = &t
	}

	// Issue the refresh token first so we can store its hash.
	refreshToken, err := s.issueToken(user.ID, user.IsAdmin, 0, s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	session, err := s.sessions.Create(ctx, &domain.Session{
		TokenHash: hashToken(refreshToken),
		UserID:    user.ID,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	accessToken, err := s.issueToken(user.ID, user.IsAdmin, session.ID, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	// Re-issue the refresh token with the real session ID now that we have it.
	refreshToken, err = s.issueToken(user.ID, user.IsAdmin, session.ID, s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

// Logout deactivates the session identified by sessionID.
func (s *AuthService) Logout(ctx context.Context, sessionID int) error {
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// Refresh validates a refresh token, issues a new token pair, and deactivates
// the old session.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, userAgent string) (*TokenPair, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	session, err := s.sessions.GetByTokenHash(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if session.ExpiresAt != nil && time.Now().After(*session.ExpiresAt) {
		_ = s.sessions.Delete(ctx, session.ID)
		return nil, domain.ErrUnauthorized
	}

	// Rotate: deactivate old session.
	if err := s.sessions.Delete(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("deactivate old session: %w", err)
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if user.IsBlocked {
		return nil, domain.ErrForbidden
	}

	var expiresAt *time.Time
	if s.refreshTTL > 0 {
		t := time.Now().Add(s.refreshTTL)
		expiresAt = &t
	}

	newRefresh, err := s.issueToken(user.ID, user.IsAdmin, 0, s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	newSession, err := s.sessions.Create(ctx, &domain.Session{
		TokenHash: hashToken(newRefresh),
		UserID:    user.ID,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	accessToken, err := s.issueToken(user.ID, user.IsAdmin, newSession.ID, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	newRefresh, err = s.issueToken(user.ID, user.IsAdmin, newSession.ID, s.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

// ListSessions returns all active sessions for the given user.
func (s *AuthService) ListSessions(ctx context.Context, userID int16, currentSessionID int) (*domain.SessionList, error) {
	list, err := s.sessions.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	for i := range list.Items {
		list.Items[i].IsCurrent = list.Items[i].ID == currentSessionID
	}
	return list, nil
}

// TerminateSession deactivates a specific session, enforcing ownership.
func (s *AuthService) TerminateSession(ctx context.Context, callerID int16, isAdmin bool, sessionID int) error {
	if !isAdmin {
		list, err := s.sessions.ListByUser(ctx, callerID)
		if err != nil {
			return fmt.Errorf("terminate session: %w", err)
		}
		owned := false
		for _, sess := range list.Items {
			if sess.ID == sessionID {
				owned = true
				break
			}
		}
		if !owned {
			return domain.ErrForbidden
		}
	}

	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("terminate session: %w", err)
	}
	return nil
}

// ParseAccessToken parses and validates an access token, returning its claims.
func (s *AuthService) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims, err := s.parseToken(tokenStr)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

// issueToken signs a JWT with the given parameters.
func (s *AuthService) issueToken(userID int16, isAdmin bool, sessionID int, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: sessionID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// parseToken verifies the signature and parses claims from a token string.
func (s *AuthService) parseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

// hashToken returns the SHA-256 hex digest of a token string.
// The raw token is never stored; only the hash goes to the database.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
