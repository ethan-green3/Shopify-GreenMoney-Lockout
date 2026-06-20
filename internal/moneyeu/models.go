package moneyeu

type PaymentS2SRequest struct {
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	OrderDescription string `json:"orderDescription"`
	Name             string `json:"name"`
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Mail             string `json:"mail"`
	DialCode         string `json:"dialCode"`
	PhoneNumber      string `json:"phoneNumber"`
	Address          string `json:"address"`
	Country          string `json:"country"`
	State            string `json:"state"`
	City             string `json:"city"`
	Zip              string `json:"zip"`
	Language         string `json:"language"`
	Sms              bool   `json:"sms"`
	CustomerService  string `json:"customerService"`
	Date             string `json:"date"`
	PaidDate         string `json:"paidDate"`
	ReturnURL        string `json:"return_url"`
	OrderIDExt       string `json:"orderidext"`
}

type PaymentS2SResponse struct {
	TransactionID string `json:"transaction_id"`
	ProcessingURL string `json:"processing_url"`
	Message       string `json:"message"`
	Status        string `json:"status"`
}
