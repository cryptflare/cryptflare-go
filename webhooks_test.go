package cryptflare

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

const webhookSecret = "whsec_super_secret_value"

func makeWebhookHeader(t *testing.T, body string, ts int64) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + body))
	return "t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookHappyPath(t *testing.T) {
	body := `{"id":"evt_1","type":"secret.rotated"}`
	ts := int64(1_700_000_000)
	header := makeWebhookHeader(t, body, ts)

	event, err := VerifyWebhook(body, header, webhookSecret, &VerifyWebhookOptions{
		Now: func() time.Time { return time.Unix(ts, 0) },
	})
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event["type"] != "secret.rotated" {
		t.Fatalf("got %v", event["type"])
	}
}

func TestVerifyWebhookMissingHeader(t *testing.T) {
	_, err := VerifyWebhook(`{}`, "", webhookSecret, nil)
	if !IsWebhookVerificationError(err) {
		t.Fatalf("expected verification error, got %v", err)
	}
}

func TestVerifyWebhookExpired(t *testing.T) {
	body := `{"id":"evt_1"}`
	ts := int64(1_700_000_000)
	header := makeWebhookHeader(t, body, ts)
	_, err := VerifyWebhook(body, header, webhookSecret, &VerifyWebhookOptions{
		Now: func() time.Time { return time.Unix(ts+600, 0) },
	})
	if !IsWebhookVerificationError(err) {
		t.Fatalf("expected verification error, got %v", err)
	}
}

func TestVerifyWebhookTampered(t *testing.T) {
	body := `{"id":"evt_1"}`
	ts := int64(1_700_000_000)
	header := makeWebhookHeader(t, body, ts)
	_, err := VerifyWebhook(body+"x", header, webhookSecret, &VerifyWebhookOptions{
		Now: func() time.Time { return time.Unix(ts, 0) },
	})
	if !IsWebhookVerificationError(err) {
		t.Fatalf("expected verification error, got %v", err)
	}
}
