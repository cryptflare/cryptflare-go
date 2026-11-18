package cryptflare

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// VerifyWebhookOptions tunes the timestamp tolerance + clock source.
type VerifyWebhookOptions struct {
	// ToleranceSeconds is the maximum drift allowed between the
	// timestamp baked into the signature and the verifier's clock.
	// Default 300 (5 minutes). Set to 0 to disable replay protection
	// (do not).
	ToleranceSeconds int
	// Now overrides the wall clock; tests use this to pin a fixed
	// instant.
	Now func() time.Time
}

// WebhookVerificationError wraps every signature failure mode. The
// `Code` field narrows: `webhook_signature_missing`,
// `webhook_signature_malformed`, `webhook_signature_expired`,
// `webhook_signature_mismatch`, `webhook_body_invalid`.
type WebhookVerificationError struct {
	Code    string
	Message string
}

func (e *WebhookVerificationError) Error() string {
	return "cryptflare: " + e.Code + ": " + e.Message
}

// VerifyWebhook checks the `X-Cryptflare-Signature` header against
// the raw body and returns the parsed event payload as
// `map[string]any`. Stripe-compatible header shape (`t=<unix>,v1=<hex>`)
// and constant-time comparison.
//
// Re-stringifying parsed JSON breaks the signature. Read the raw body
// from your server's request before parsing.
func VerifyWebhook(body, header, secret string, opts *VerifyWebhookOptions) (map[string]any, error) {
	if header == "" {
		return nil, &WebhookVerificationError{Code: "webhook_signature_missing", Message: "missing X-Cryptflare-Signature header"}
	}
	o := VerifyWebhookOptions{ToleranceSeconds: 300}
	if opts != nil {
		if opts.ToleranceSeconds != 0 {
			o.ToleranceSeconds = opts.ToleranceSeconds
		}
		if opts.Now != nil {
			o.Now = opts.Now
		}
	}

	timestamp, signatures, parseErr := parseSignatureHeader(header)
	if parseErr != nil {
		return nil, parseErr
	}
	if len(signatures) == 0 {
		return nil, &WebhookVerificationError{Code: "webhook_signature_malformed", Message: "no v1 signatures in header"}
	}

	if o.ToleranceSeconds > 0 {
		var nowSec int64
		if o.Now != nil {
			nowSec = o.Now().Unix()
		} else {
			nowSec = time.Now().Unix()
		}
		drift := nowSec - timestamp
		if drift < 0 {
			drift = -drift
		}
		if drift > int64(o.ToleranceSeconds) {
			return nil, &WebhookVerificationError{
				Code:    "webhook_signature_expired",
				Message: "timestamp outside tolerance window",
			}
		}
	}

	signed := strconv.FormatInt(timestamp, 10) + "." + body
	expected := hmacSha256Hex(secret, signed)
	matched := false
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, &WebhookVerificationError{Code: "webhook_signature_mismatch", Message: "signature does not match payload"}
	}

	out := map[string]any{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, &WebhookVerificationError{Code: "webhook_body_invalid", Message: "body is not valid JSON"}
	}
	return out, nil
}

func parseSignatureHeader(header string) (int64, []string, error) {
	var timestamp int64
	var signatures []string
	for _, segment := range strings.Split(header, ",") {
		idx := strings.IndexByte(segment, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(segment[:idx])
		value := strings.TrimSpace(segment[idx+1:])
		switch key {
		case "t":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, &WebhookVerificationError{Code: "webhook_signature_malformed", Message: "non-integer timestamp"}
			}
			timestamp = n
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == 0 {
		return 0, nil, &WebhookVerificationError{Code: "webhook_signature_malformed", Message: "missing t= timestamp"}
	}
	return timestamp, signatures, nil
}

func hmacSha256Hex(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// IsWebhookVerificationError reports whether err (or any wrapped
// cause) is a `*WebhookVerificationError`.
func IsWebhookVerificationError(err error) bool {
	var w *WebhookVerificationError
	return errors.As(err, &w)
}
