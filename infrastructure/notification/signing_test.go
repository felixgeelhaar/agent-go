package notification

import (
	"testing"
	"time"
)

func TestSigner_SignPayload(t *testing.T) {
	signer := NewSigner()

	tests := []struct {
		name    string
		payload []byte
		secret  string
	}{
		{
			name:    "simple payload",
			payload: []byte(`{"event":"test"}`),
			secret:  "test-secret",
		},
		{
			name:    "empty payload",
			payload: []byte{},
			secret:  "test-secret",
		},
		{
			name:    "complex payload",
			payload: []byte(`{"events":[{"type":"run.started","payload":{"run_id":"abc123"}}]}`),
			secret:  "super-secret-key-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := signer.SignPayload(tt.payload, tt.secret)

			// Signature should start with algorithm prefix
			if len(signature) < 7 || signature[:7] != "sha256=" {
				t.Errorf("signature should start with 'sha256=', got: %s", signature)
			}

			// Signature should be consistent
			signature2 := signer.SignPayload(tt.payload, tt.secret)
			if signature != signature2 {
				t.Errorf("signature should be deterministic, got %s and %s", signature, signature2)
			}

			// Different secrets should produce different signatures
			signature3 := signer.SignPayload(tt.payload, "different-secret")
			if signature == signature3 {
				t.Error("different secrets should produce different signatures")
			}
		})
	}
}

func TestSigner_VerifySignature(t *testing.T) {
	signer := NewSigner()

	tests := []struct {
		name      string
		payload   []byte
		secret    string
		signature string
		wantValid bool
	}{
		{
			name:      "valid signature",
			payload:   []byte(`{"event":"test"}`),
			secret:    "test-secret",
			signature: "", // Will be computed
			wantValid: true,
		},
		{
			name:      "invalid signature",
			payload:   []byte(`{"event":"test"}`),
			secret:    "test-secret",
			signature: "sha256=invalid",
			wantValid: false,
		},
		{
			name:      "wrong secret",
			payload:   []byte(`{"event":"test"}`),
			secret:    "wrong-secret",
			signature: "", // Will be computed with different secret
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := tt.signature
			if signature == "" {
				if tt.name == "valid signature" {
					signature = signer.SignPayload(tt.payload, tt.secret)
				} else {
					signature = signer.SignPayload(tt.payload, "original-secret")
				}
			}

			valid := signer.VerifySignature(tt.payload, tt.secret, signature)
			if valid != tt.wantValid {
				t.Errorf("VerifySignature() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestSigner_SignedHeaders(t *testing.T) {
	signer := NewSigner()

	payload := []byte(`{"event":"test"}`)
	secret := "test-secret"
	timestamp := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC

	headers := signer.SignedHeaders(payload, secret, timestamp)

	// Should have all three headers
	requiredHeaders := []string{
		"X-Webhook-Signature",
		"X-Webhook-Timestamp",
		"X-Webhook-Signature-V2",
	}

	for _, h := range requiredHeaders {
		if _, ok := headers[h]; !ok {
			t.Errorf("missing required header: %s", h)
		}
	}

	// Timestamp header should match
	if headers["X-Webhook-Timestamp"] != "1609459200" {
		t.Errorf("timestamp header = %s, want 1609459200", headers["X-Webhook-Timestamp"])
	}

	// Signature headers should have correct format
	if len(headers["X-Webhook-Signature"]) < 7 || headers["X-Webhook-Signature"][:7] != "sha256=" {
		t.Errorf("signature should start with 'sha256='")
	}
	if len(headers["X-Webhook-Signature-V2"]) < 7 || headers["X-Webhook-Signature-V2"][:7] != "sha256=" {
		t.Errorf("signature-v2 should start with 'sha256='")
	}
}

func TestSigner_VerifyTimestampedSignature(t *testing.T) {
	signer := NewSigner()

	payload := []byte(`{"event":"test"}`)
	secret := "test-secret"
	now := time.Now()
	tolerance := 5 * time.Minute

	// Create valid timestamped signature
	headers := signer.SignedHeaders(payload, secret, now)
	signature := headers["X-Webhook-Signature-V2"]

	tests := []struct {
		name      string
		timestamp int64
		signature string
		wantValid bool
	}{
		{
			name:      "valid recent signature",
			timestamp: now.Unix(),
			signature: signature,
			wantValid: true,
		},
		{
			name:      "expired signature",
			timestamp: now.Add(-10 * time.Minute).Unix(),
			signature: signature,
			wantValid: false,
		},
		{
			name:      "future signature beyond tolerance",
			timestamp: now.Add(10 * time.Minute).Unix(),
			signature: signature,
			wantValid: false,
		},
		{
			name:      "invalid signature",
			timestamp: now.Unix(),
			signature: "sha256=invalid",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := signer.VerifyTimestampedSignature(payload, secret, tt.signature, tt.timestamp, tolerance)
			if valid != tt.wantValid {
				t.Errorf("VerifyTimestampedSignature() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}
