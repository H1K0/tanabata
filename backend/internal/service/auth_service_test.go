package service

import (
	"testing"
	"time"
)

// newContentTokenService builds an AuthService for content-token tests. The
// content-token methods never touch the user/session repos, so nil is fine.
func newContentTokenService(contentTTL time.Duration) *AuthService {
	return NewAuthService(nil, nil, "test-secret", 15*time.Minute, 720*time.Hour, contentTTL)
}

func TestContentTokenRoundTrip(t *testing.T) {
	s := newContentTokenService(time.Hour)
	const fid = "11111111-1111-1111-1111-111111111111"

	tok, expiresIn, err := s.GenerateContentToken(fid, 7, true)
	if err != nil {
		t.Fatalf("GenerateContentToken: %v", err)
	}
	if expiresIn != int(time.Hour.Seconds()) {
		t.Fatalf("expires_in = %d, want %d", expiresIn, int(time.Hour.Seconds()))
	}

	claims, err := s.ValidateContentToken(tok, fid)
	if err != nil {
		t.Fatalf("ValidateContentToken: %v", err)
	}
	if claims.UserID != 7 || !claims.IsAdmin {
		t.Fatalf("claims user mismatch: uid=%d adm=%v", claims.UserID, claims.IsAdmin)
	}
	if claims.FileID != fid || claims.TokenType != tokenTypeContent {
		t.Fatalf("claims scope mismatch: fid=%q typ=%q", claims.FileID, claims.TokenType)
	}
}

func TestContentTokenRejectsOtherFile(t *testing.T) {
	s := newContentTokenService(time.Hour)
	tok, _, err := s.GenerateContentToken("11111111-1111-1111-1111-111111111111", 7, false)
	if err != nil {
		t.Fatal(err)
	}
	// A token minted for one file must not authorize another.
	if _, err := s.ValidateContentToken(tok, "22222222-2222-2222-2222-222222222222"); err == nil {
		t.Fatal("expected rejection for a different file id")
	}
}

func TestContentTokenRejectsAccessToken(t *testing.T) {
	s := newContentTokenService(time.Hour)
	// An ordinary access token must not pass as a content token (wrong type).
	access, err := s.issueToken(7, false, 1, 15*time.Minute, tokenTypeAccess)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateContentToken(access, ""); err == nil {
		t.Fatal("expected rejection of an access token as a content token")
	}
}

func TestContentTokenRejectsExpired(t *testing.T) {
	// Negative TTL → the token is already expired when minted.
	s := newContentTokenService(-time.Minute)
	const fid = "11111111-1111-1111-1111-111111111111"
	tok, _, err := s.GenerateContentToken(fid, 7, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateContentToken(tok, fid); err == nil {
		t.Fatal("expected rejection of an expired content token")
	}
}

func TestContentTokenRejectsGarbage(t *testing.T) {
	s := newContentTokenService(time.Hour)
	if _, err := s.ValidateContentToken("not-a-jwt", "11111111-1111-1111-1111-111111111111"); err == nil {
		t.Fatal("expected rejection of a malformed token")
	}
}
