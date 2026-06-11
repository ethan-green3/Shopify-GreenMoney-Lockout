package filevine

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type ObjectID struct {
	ProjectTypeID   int64  `json:"ProjectTypeId"`
	SectionSelector string `json:"SectionSelector"`
	ItemID          string `json:"ItemId"`
}

type CollectionItemUpdated struct {
	ObjectID  ObjectID       `json:"ObjectId"`
	Other     map[string]any `json:"Other"`
	Timestamp int64          `json:"Timestamp"`
	Object    string         `json:"Object"`
	Event     string         `json:"Event"`
	OrgID     int64          `json:"OrgId"`
	ProjectID int64          `json:"ProjectId"`
	UserID    int64          `json:"UserId"`
	UserType  string         `json:"UserType"`
}

func CollectionItemUpdatedHandler(w http.ResponseWriter, r *http.Request) {
	var collectionItemUpdated CollectionItemUpdated
	if err := json.NewDecoder(r.Body).Decode(&collectionItemUpdated); err != nil {
		log.Println("Filevine collectionItemUpdated error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	log.Println("Filevine collectionItemUpdated Payload ", collectionItemUpdated)
	switch collectionItemUpdated.ObjectID.SectionSelector {
	case "meds":
		callMedsWebhookInZapier(collectionItemUpdated)
		return
	default:
		log.Println("Filevine collectionItemUpdated we don't care about: ", collectionItemUpdated.ObjectID.SectionSelector)
		return
	}

}

func callMedsWebhookInZapier(collectionItemUpdated CollectionItemUpdated) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	jsonData, err := json.Marshal(collectionItemUpdated)
	if err != nil {
		log.Println("Filevine collectionItemUpdated error", err)
		return
	}
	req, err := http.NewRequest("POST", os.Getenv("ZAPIER_WEBHOOK_URL"), bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Filevine collectionItemUpdated error", err)
		return
	}
	res, err := client.Do(req)
	if err != nil {
		log.Println("Filevine collectionItemUpdated error", err)
		return
	}
	defer res.Body.Close()
	log.Println("Filevine collectionItemUpdated response", res)
}
