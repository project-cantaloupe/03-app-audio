package authn

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	testIssuer   = "https://identity.example.com/realms/cantaloupe"
	testAudience = "audio-api"
)

func TestDisabledIgnoresAllCredentials(t *testing.T) {
	subject, err := (Disabled{}).Subject(
		context.Background(),
		"Bearer ignored",
		"forged-development-user",
	)
	if err != nil {
		t.Fatalf("resolve disabled subject: %v", err)
	}
	if subject != "" {
		t.Fatalf("subject = %q, want anonymous", subject)
	}
}

func TestDevelopmentSubject(t *testing.T) {
	subject, err := (Development{}).Subject(context.Background(), "Bearer ignored", "  local-user  ")
	if err != nil {
		t.Fatalf("resolve development subject: %v", err)
	}
	if subject != "local-user" {
		t.Fatalf("subject = %q, want local-user", subject)
	}
}

func TestOIDCSubject(t *testing.T) {
	authenticator, privateKey := newTestOIDC(t)
	now := time.Now()

	tests := []struct {
		name          string
		authorization string
		wantSubject   string
		wantError     bool
	}{
		{name: "anonymous", wantSubject: ""},
		{name: "malformed header", authorization: "Basic credentials", wantError: true},
		{
			name:          "valid token",
			authorization: "Bearer " + signJWT(t, privateKey, claims(now, testIssuer, testAudience, "user-123")),
			wantSubject:   "user-123",
		},
		{
			name:          "case insensitive bearer",
			authorization: "bearer " + signJWT(t, privateKey, claims(now, testIssuer, testAudience, "user-456")),
			wantSubject:   "user-456",
		},
		{
			name:          "wrong issuer",
			authorization: "Bearer " + signJWT(t, privateKey, claims(now, "https://attacker.example.com", testAudience, "user-123")),
			wantError:     true,
		},
		{
			name:          "wrong audience",
			authorization: "Bearer " + signJWT(t, privateKey, claims(now, testIssuer, "other-api", "user-123")),
			wantError:     true,
		},
		{
			name: "expired token",
			authorization: "Bearer " + signJWT(t, privateKey, map[string]any{
				"iss": testIssuer, "aud": testAudience, "sub": "user-123",
				"iat": now.Add(-2 * time.Hour).Unix(), "exp": now.Add(-time.Hour).Unix(),
			}),
			wantError: true,
		},
		{
			name:          "missing subject",
			authorization: "Bearer " + signJWT(t, privateKey, claims(now, testIssuer, testAudience, "")),
			wantError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subject, err := authenticator.Subject(context.Background(), test.authorization, "forged-development-user")
			if test.wantError {
				if !errors.Is(err, ErrInvalidCredentials) {
					t.Fatalf("error = %v, want ErrInvalidCredentials", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve subject: %v", err)
			}
			if subject != test.wantSubject {
				t.Fatalf("subject = %q, want %q", subject, test.wantSubject)
			}
		})
	}
}

func newTestOIDC(t *testing.T) (*OIDC, *rsa.PrivateKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	verifier := oidc.NewVerifier(testIssuer, &oidc.StaticKeySet{
		PublicKeys: []crypto.PublicKey{&privateKey.PublicKey},
	}, &oidc.Config{
		ClientID:             testAudience,
		SupportedSigningAlgs: []string{"RS256"},
	})
	return &OIDC{verifier: verifier}, privateKey
}

func claims(now time.Time, issuer, audience, subject string) map[string]any {
	return map[string]any{
		"iss": issuer,
		"aud": audience,
		"sub": subject,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

func signJWT(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal JWT claims: %v", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
