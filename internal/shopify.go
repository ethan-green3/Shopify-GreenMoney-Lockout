package internal

// ShopifyOrder is the subset of the Shopify orders/create webhook
// payload that we care about for this POC.
type ShopifyOrder struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	TotalPrice          string   `json:"total_price"`
	Currency            string   `json:"currency"`
	PaymentGatewayNames []string `json:"payment_gateway_names"`
}

// IsGreenMoneyOrder returns true if the order used the "Green Money" payment method.
func IsGreenMoneyOrder(o ShopifyOrder) bool {
	for _, name := range o.PaymentGatewayNames {
		if name == "Green Money" {
			return true
		}
	}
	return false
}
