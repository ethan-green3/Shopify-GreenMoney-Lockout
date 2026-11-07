package burner

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
)

func TestParseSignatureAndDigest(t *testing.T) {
	body := []byte(`{"event":"invoice.swept"}`)
	ts := int64(1700000000)
	secret := "webhook-secret"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(body)))
	got, provided, err := parseSignature("t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil)))
	if err != nil || got != ts || len(provided) != sha256.Size {
		t.Fatalf("parse failed: %v", err)
	}
}

func TestParseSignatureRejectsMalformed(t *testing.T) {
	for _, value := range []string{"", "t=1", "v1=00", "t=1,t=2,v1=00", "t=1,v1=xx", "t=1,v1=00,foo=bar"} {
		if _, _, err := parseSignature(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}
