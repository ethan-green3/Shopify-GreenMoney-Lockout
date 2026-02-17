package moneyeu

import "encoding/json"

type CreateOrderExtRequest struct {
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Name            string  `json:"name"`
	Mail            string  `json:"mail"`
	PhoneNumber     string  `json:"phoneNumber"`
	DialCode        string  `json:"dialCode"`
	Address         string  `json:"address"`
	City            string  `json:"city"`
	State           string  `json:"state"`
	Zip             string  `json:"zip"`
	Country         string  `json:"country"`
	IdOrderExt      string  `json:"idOrderExt"`
	Language        string  `json:"language"`
	Sms             bool    `json:"sms"`
	CustomerService string  `json:"customerService"`
}

type CreateOrderExtResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Content json.RawMessage `json:"content"` // can be object OR array
}

type CreateOrderExtContent struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	IdOrderExt string `json:"idOrderExt"`
	Url        string `json:"url"`
}
