package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// Token types distinguish short-lived access tokens from long-lived refresh
// tokens so the two cannot be substituted for one another.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// dummyPasswordHash is a valid bcrypt hash used to equalise the cost of a login
// attempt against a non-existent user, preventing username enumeration via
// response timing. It is the hash of a random string no one knows.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Claims is the JWT payload for both access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	UserID    int16  `json:"uid"`
	IsAdmin   bool   `json:"adm"`
	SessionID int    `json:"sid"`
	TokenType string `json:"typ"`
}

// TokenPair holds an issued access/refresh token pair with the access TTL.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // access token TTL in seconds
}

// AuthService handles authentication and session lifecycle.
type AuthService struct {
	users      port.UserRepo
	sessions   port.SessionRepo
	secret     []byte
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
		// Compare against a dummy hash so a missing user costs the same as a
		// wrong password, and return ErrUnauthorized either way to avoid
		// username enumeration.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return nil, domain.ErrUnauthorized
	}

	// Verify the password before disclosing anything about account state, so a
	// caller cannot distinguish "blocked" from "wrong password".
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	if user.IsBlocked {
		return nil, domain.ErrForbidden
	}

	return s.issuePair(ctx, user, userAgent)
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
	if err != nil || claims.TokenType != tokenTypeRefresh {
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

	return s.issuePair(ctx, user, userAgent)
}

// issuePair creates a session and the access/refresh token pair for user.
//
// The refresh token is issued first and its hash is stored as the session's
// identity; the refresh token is located on /refresh purely by that hash, so it
// carries no session ID. The access token then embeds the real session ID so it
// can be revoked on logout. Because the stored hash is the hash of the token
// actually returned, /refresh works (unlike the previous re-issue approach).
func (s *AuthService) issuePair(ctx context.Context, user *domain.User, userAgent string) (*TokenPair, error) {
	var expiresAt *time.Time
	if s.refreshTTL > 0 {
		t := time.Now().Add(s.refreshTTL)
		expiresAt = &t
	}

	refreshToken, err := s.issueToken(user.ID, user.IsAdmin, 0, s.refreshTTL, tokenTypeRefresh)
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

	accessToken, err := s.issueToken(user.ID, user.IsAdmin, session.ID, s.accessTTL, tokenTypeAccess)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
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

// ValidateAccessToken parses and validates an access token, returning its
// claims. A refresh token is rejected (wrong type), and the token's session
// must still be active — so logout, session termination, an admin block, or a
// refresh rotation revoke any outstanding access tokens immediately rather than
// only at expiry.
func (s *AuthService) ValidateAccessToken(ctx context.Context, tokenStr string) (*Claims, error) {
	claims, err := s.parseToken(tokenStr)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	if claims.TokenType != tokenTypeAccess {
		return nil, domain.ErrUnauthorized
	}
	if _, err := s.sessions.GetByID(ctx, claims.SessionID); err != nil {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

// issueToken signs a JWT with the given parameters. A random JWT ID guarantees
// uniqueness even for tokens minted within the same second.
func (s *AuthService) issueToken(userID int16, isAdmin bool, sessionID int, ttl time.Duration, tokenType string) (string, error) {
	jti, err := randomJTI()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: sessionID,
		TokenType: tokenType,
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

// randomJTI returns a 128-bit random hex string for use as a JWT ID.
func randomJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	return hex.EncodeToString(b), nil
}
