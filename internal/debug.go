package internal

import (
	"log"
	"os"
)

func logDebugWebhookBody(kind string, raw []byte) {
	if os.Getenv("APP_ENV") == "development" && os.Getenv("DEBUG_LOG_WEBHOOK_BODIES") == "1" {
		log.Printf("DEBUG %s request body: %s", kind, string(raw))
	}
}
