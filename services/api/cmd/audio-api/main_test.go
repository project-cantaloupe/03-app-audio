package main

import (
	"strings"
	"testing"
)

func TestLoadConfigAuthentication(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		issuer   string
		audience string
		wantErr  string
	}{
		{name: "development", mode: "development"},
		{
			name: "oidc",
			mode: "oidc", issuer: "https://identity.example.com/realms/cantaloupe", audience: "audio-api",
		},
		{name: "oidc missing issuer", mode: "oidc", audience: "audio-api", wantErr: "OIDC_ISSUER_URL and OIDC_AUDIENCE"},
		{name: "unsupported mode", mode: "invalid", wantErr: "AUTH_MODE must be development or oidc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("AUTH_MODE", test.mode)
			t.Setenv("OIDC_ISSUER_URL", test.issuer)
			t.Setenv("OIDC_AUDIENCE", test.audience)

			config, err := loadConfig()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want text %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if config.AuthMode != test.mode || config.OIDCIssuerURL != test.issuer || config.OIDCAudience != test.audience {
				t.Fatalf("unexpected authentication config: %+v", config)
			}
		})
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://audio@localhost/audio")
	t.Setenv("AWS_REGION", "ap-northeast-2")
	t.Setenv("QUARANTINE_BUCKET", "cntlp-aws-quarantine")
	t.Setenv("ARTIFACT_BUCKET", "cntlp-aws-transcode")
	t.Setenv("SCAN_RESULT_QUEUE_URL", "https://sqs.example.com/scan-result")
	t.Setenv("PLAYBACK_URL_MODE", "s3")
}
