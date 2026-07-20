package moneyeu

type ProcessPaymentRequest struct {
	CustomerName       string  `json:"customerName"`
	Address            string  `json:"address,omitempty"`
	Zip                string  `json:"zip,omitempty"`
	MerchantTerminalID any     `json:"merchantTerminalId,omitempty"`
	CallbackURL        string  `json:"callbackUrl,omitempty"`
	RedirectURL        string  `json:"redirectUrl,omitempty"`
	City               string  `json:"city,omitempty"`
	State              string  `json:"state,omitempty"`
	CountryName        string  `json:"countryName,omitempty"`
	CustomerEmail      string  `json:"customerEmail"`
	Phone              string  `json:"phone,omitempty"`
	Amount             float64 `json:"amount"`
	Currency           string  `json:"currency"`
	CardNumber         string  `json:"cardNumber,omitempty"`
	CardholderName     string  `json:"cardholderName,omitempty"`
	ExpiryDate         string  `json:"expiryDate,omitempty"`
	CVV                string  `json:"cvv,omitempty"`
	MerchantName       string  `json:"merchantName,omitempty"`

	Language string `json:"language,omitempty"`
	Service  string `json:"service,omitempty"`

	// MoneyEU allows merchant-specific fields in the payment body. Keep these
	// stable so callbacks can still be correlated to Shopify orders.
	OrderIDExt string `json:"orderIDExt"`

	StoreFrontURL     string `json:"storeFrontUrl"`
	CustomerIPAddress string `json:"customerIpAddress"`
	CustomerUserAgent string `json:"customerUserAgent"`
}

type ProcessPaymentResponse struct {
	Success   bool                       `json:"success"`
	Message   string                     `json:"message"`
	Data      ProcessPaymentResponseData `json:"data"`
	ErrorCode string                     `json:"errorCode,omitempty"`
	Status    int                        `json:"status,omitempty"`
	Path      string                     `json:"path,omitempty"`
	Timestamp string                     `json:"timestamp,omitempty"`
}

type ProcessPaymentResponseData struct {
	OrderID           string `json:"orderId"`
	TransactionID     string `json:"transactionId"`
	TransactionStatus string `json:"transactionStatus"`
	FlowType          string `json:"flowType"`
	HPP               bool   `json:"hpp"`
	PaymentURL        string `json:"paymentUrl"`
	RedirectURL       string `json:"redirectUrl"`
	CheckoutToken     string `json:"checkoutToken"`
	CheckoutURL       string `json:"checkoutUrl"`
}
