package burner

import (
	"fmt"
	"net/url"
	"strings"
)

type CheckoutFields struct {
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	City         string `json:"address_city"`
	State        string `json:"address_state"`
	PostalCode   string `json:"address_postal_code"`
	Country      string `json:"address_country"`
	Source       string `json:"source"`
	ReturnURL    string `json:"return_url"`
}

func CheckoutURL(base, invoiceID string, f CheckoutFields) (string, error) {
	base = strings.TrimSpace(base)
	if !strings.HasPrefix(base, "https://") {
		return "", fmt.Errorf("Burner checkout base URL must use https")
	}
	joined, err := url.JoinPath(strings.TrimRight(base, "/"), "pay", invoiceID)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(joined)
	if err != nil {
		return "", err
	}
	q := u.Query()
	set := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			q.Set(key, strings.TrimSpace(value))
		}
	}
	set("phone", f.Phone)
	set("email", f.Email)
	set("first_name", f.FirstName)
	set("last_name", f.LastName)
	set("address_line1", f.AddressLine1)
	set("address_line2", f.AddressLine2)
	set("address_city", f.City)
	set("address_state", f.State)
	set("address_postal_code", f.PostalCode)
	set("address_country", f.Country)
	set("source", f.Source)
	set("return_url", f.ReturnURL)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
