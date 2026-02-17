package internal

// ShopifyAddress is a subset of the address object from Shopify.
type ShopifyAddress struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// You can add more fields later if you want (address1, city, etc.)
}

// ShopifyCustomer is a subset of the customer object from Shopify.
type ShopifyCustomer struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// ShopifyOrder is the subset of the Shopify orders/create webhook
// payload that we care about for this POC.
type ShopifyOrder struct {
	ID                  int64            `json:"id"`
	Name                string           `json:"name"`
	Email               string           `json:"email"`
	TotalPrice          string           `json:"total_price"`
	Currency            string           `json:"currency"`
	PaymentGatewayNames []string         `json:"payment_gateway_names"`
	BillingAddress      *ShopifyAddress  `json:"billing_address"`
	ShippingAddress     *ShopifyAddress  `json:"shipping_address"`
	Customer            *ShopifyCustomer `json:"customer"`
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

func IsMoneyEUOrder(o ShopifyOrder) bool {
	for _, name := range o.PaymentGatewayNames {
		if name == "Credit/Debit Card" {
			return true
		}
	}
	return false
}
