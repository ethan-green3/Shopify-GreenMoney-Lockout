package burner

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type webhookEnvelope struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Data      struct {
		InvoiceID string `json:"invoiceId"`
		Status    string `json:"status"`
	} `json:"data"`
}

func WebhookHandler(store *Store, secret string, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(secret) == "" {
			http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if os.Getenv("DEBUG_LOG_WEBHOOK_BODIES") == "1" {
			log.Printf("DEBUG Burner webhook request body: %s", string(raw))
		}
		timestamp, provided, err := parseSignature(r.Header.Get("X-Burner-Signature"))
		if err != nil {
			http.Error(w, "invalid signature", 401)
			return
		}
		if abs(now().Unix()-timestamp) > 300 {
			http.Error(w, "stale signature", 401)
			return
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "." + string(raw)))
		if len(provided) != sha256.Size || subtle.ConstantTimeCompare(provided, mac.Sum(nil)) != 1 {
			http.Error(w, "invalid signature", 401)
			return
		}
		var event webhookEnvelope
		if err := json.Unmarshal(raw, &event); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		fpBytes := sha256.Sum256([]byte(r.Header.Get("X-Burner-Signature") + "\n" + string(raw)))
		created, err := store.SaveWebhook(r.Context(), hex.EncodeToString(fpBytes[:]), event.Event, timestamp, event.Data.InvoiceID, raw)
		if err != nil {
			http.Error(w, "temporary failure", 503)
			return
		}
		if created && event.Data.InvoiceID != "" {
			if payment, lookupErr := store.FindPaymentByInvoiceID(r.Context(), event.Data.InvoiceID); lookupErr == nil {
				if enqueueErr := store.Enqueue(r.Context(), payment.ID, JobReconcileInvoice, time.Now(), nil); enqueueErr != nil {
					http.Error(w, "temporary failure", http.StatusServiceUnavailable)
					return
				}
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func parseSignature(header string) (int64, []byte, error) {
	var ts string
	var sig string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return 0, nil, fmt.Errorf("malformed signature")
		}
		switch kv[0] {
		case "t":
			if ts != "" {
				return 0, nil, fmt.Errorf("duplicate timestamp")
			}
			ts = kv[1]
		case "v1":
			if sig != "" {
				return 0, nil, fmt.Errorf("duplicate signature")
			}
			sig = kv[1]
		default:
			return 0, nil, fmt.Errorf("unknown signature component")
		}
	}
	if ts == "" || sig == "" {
		return 0, nil, fmt.Errorf("missing signature component")
	}
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0, nil, err
	}
	decoded, err := hex.DecodeString(sig)
	if err != nil || len(decoded) != sha256.Size {
		return 0, nil, fmt.Errorf("invalid signature length")
	}
	return timestamp, decoded, nil
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
