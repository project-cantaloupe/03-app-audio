package authn

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const providerRequestTimeout = 10 * time.Second

type OIDC struct {
	verifier *oidc.IDTokenVerifier
}

// NewOIDC discovers Keycloak's metadata and JWKS endpoint from the issuer.
// The verifier caches signing keys and refreshes them when Keycloak rotates a key.
func NewOIDC(ctx context.Context, issuerURL, audience string) (*OIDC, error) {
	issuerURL = strings.TrimSuffix(strings.TrimSpace(issuerURL), "/")
	audience = strings.TrimSpace(audience)
	if issuerURL == "" || audience == "" {
		return nil, fmt.Errorf("configure OIDC authenticator: OIDC_ISSUER_URL and OIDC_AUDIENCE are required")
	}

	providerContext := oidc.ClientContext(ctx, &http.Client{Timeout: providerRequestTimeout})
	provider, err := oidc.NewProvider(providerContext, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}

	return &OIDC{
		verifier: provider.VerifierContext(providerContext, &oidc.Config{
			ClientID: audience,
		}),
	}, nil
}

func (authenticator *OIDC) Subject(ctx context.Context, authorizationHeader, _ string) (string, error) {
	rawToken, present, err := bearerToken(authorizationHeader)
	if err != nil {
		return "", err
	}
	if !present {
		return "", nil
	}

	token, err := authenticator.verifier.Verify(ctx, rawToken)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	subject := strings.TrimSpace(token.Subject)
	if subject == "" {
		return "", ErrInvalidCredentials
	}
	return subject, nil
}

func bearerToken(header string) (token string, present bool, err error) {
	if strings.TrimSpace(header) == "" {
		return "", false, nil
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false, ErrInvalidCredentials
	}
	return parts[1], true, nil
}
