package authn

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidCredentials = errors.New("invalid authentication credentials")

// Authenticator resolves the immutable user subject for one HTTP request.
// An empty subject without an error means that the request is anonymous.
type Authenticator interface {
	Subject(ctx context.Context, authorizationHeader, developmentSubjectHeader string) (string, error)
}

// Development trusts the local-only subject header. It must never be selected
// for a publicly reachable deployment.
type Development struct{}

func (Development) Subject(_ context.Context, _ string, developmentSubjectHeader string) (string, error) {
	return strings.TrimSpace(developmentSubjectHeader), nil
}
