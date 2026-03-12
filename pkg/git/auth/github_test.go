package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueJWTFromPEM(t *testing.T) {
	// Generate a test RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	appID := "test-app-123"

	// Generate JWT
	tokenString, err := issueJWTFromPEM(privateKey, appID)
	if err != nil {
		t.Fatalf("Failed to issue JWT: %v", err)
	}

	if tokenString == "" {
		t.Fatal("Expected non-empty token string")
	}

	// Parse and validate the JWT
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			t.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return &privateKey.PublicKey, nil
	})

	if err != nil {
		t.Fatalf("Failed to parse JWT: %v", err)
	}

	if !token.Valid {
		t.Fatal("Token is not valid")
	}

	// Validate claims
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		t.Fatal("Failed to get claims")
	}

	if claims.Issuer != appID {
		t.Errorf("Expected issuer %s, got %s", appID, claims.Issuer)
	}

	// Validate IssuedAt is in the past (with 1 minute tolerance)
	now := time.Now()
	if claims.IssuedAt == nil {
		t.Fatal("IssuedAt claim is nil")
	}
	issuedAt := claims.IssuedAt.Time
	if issuedAt.After(now) {
		t.Errorf("IssuedAt is in the future: %v (now: %v)", issuedAt, now)
	}
	// Should be issued about 1 minute ago
	if now.Sub(issuedAt) > 2*time.Minute {
		t.Errorf("IssuedAt is too far in the past: %v (now: %v)", issuedAt, now)
	}

	// Validate ExpiresAt is in the future (about 5 minutes from now)
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt claim is nil")
	}
	expiresAt := claims.ExpiresAt.Time
	if expiresAt.Before(now) {
		t.Errorf("ExpiresAt is in the past: %v (now: %v)", expiresAt, now)
	}
	// Should expire in about 5 minutes
	if expiresAt.Sub(now) > 6*time.Minute {
		t.Errorf("ExpiresAt is too far in the future: %v (now: %v)", expiresAt, now)
	}
	if expiresAt.Sub(now) < 4*time.Minute {
		t.Errorf("ExpiresAt is too soon: %v (now: %v)", expiresAt, now)
	}
}

func TestIssueJWTFromPEM_SigningMethodIsRS256(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	tokenString, err := issueJWTFromPEM(privateKey, "test-app")
	if err != nil {
		t.Fatalf("Failed to issue JWT: %v", err)
	}

	// Parse without validation to check the header
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &jwt.RegisteredClaims{})
	if err != nil {
		t.Fatalf("Failed to parse JWT: %v", err)
	}

	if token.Method.Alg() != "RS256" {
		t.Errorf("Expected RS256 signing method, got %s", token.Method.Alg())
	}
}
