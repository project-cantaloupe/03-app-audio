package artifacturl

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCloudFrontSignerUsesSHA256CannedPolicy(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "cloudfront-private.pem")
	keyBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyBlock, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	signer, err := NewCloudFrontSigner("https://media.example.test/private", "KTEST", keyPath)
	if err != nil {
		t.Fatalf("NewCloudFrontSigner() error = %v", err)
	}
	signedURL, err := signer.Sign(context.Background(), "audios/audio-id/playback.mp3", time.Unix(1785902400, 0))
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	if parsed.Path != "/private/audios/audio-id/playback.mp3" {
		t.Fatalf("unexpected path: %s", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("Key-Pair-Id") != "KTEST" || query.Get("Hash-Algorithm") != "SHA256" || query.Get("Signature") == "" {
		t.Fatalf("signed URL query is incomplete: %s", parsed.RawQuery)
	}
}
