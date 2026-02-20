package moneyeu

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"Shopify-GreenMoney-Lockout/internal/email"
)

type ShopifyOrderLite struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	TotalPrice          string   `json:"total_price"`
	Currency            string   `json:"currency"`
	PaymentGatewayNames []string `json:"payment_gateway_names"`

	BillingAddress  *ShopifyAddressLite `json:"billing_address"`
	ShippingAddress *ShopifyAddressLite `json:"shipping_address"`
}

type ShopifyAddressLite struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	Address1    string `json:"address1"`
	City        string `json:"city"`
	Province    string `json:"province"`
	Zip         string `json:"zip"`
	CountryCode string `json:"country_code"`
}

type Service struct {
	DB     *sql.DB
	Client *Client
	SMTP   email.SMTPConfig
}

func (s *Service) HandleShopifyOrderJSON(ctx context.Context, raw []byte) error {
	var o ShopifyOrderLite
	if err := json.Unmarshal(raw, &o); err != nil {
		return fmt.Errorf("moneyeu: decode Shopify order: %w", err)
	}

	amount, err := strconv.ParseFloat(o.TotalPrice, 64)
	if err != nil {
		return fmt.Errorf("parse total_price %q: %w", o.TotalPrice, err)
	}

	// choose address
	addr := o.ShippingAddress
	if addr == nil {
		addr = o.BillingAddress
	}

	customerName := ""
	customerPhone := ""
	address1 := ""
	city := ""
	state := ""
	zip := ""
	country := ""

	if addr != nil {
		customerPhone = addr.Phone
		address1 = addr.Address1
		city = addr.City
		state = addr.Province
		zip = addr.Zip
		country = addr.CountryCode
		customerName = strings.TrimSpace(addr.FirstName + " " + addr.LastName)
	}
	// MoneyEU API Expectes X, Shopify sends down Y, update dial code for non North American orders as well
	dialCode := "+1"
	if country == "US" {
		country = "United States"
	}
	if country == "SV" {
		dialCode = "+503"
		country = "El Salvador"
	}
	if country == "CA" {
		country = "Canada"
	}
	if country == "CO" {
		dialCode = "+57"
		country = "Colombia"
	}
	//*****COMMENT THIS OUT FOR PRODUCTION************
	//o.Currency = "EUR"
	// 1) Insert DB row first
	paymentID, err := InsertMoneyEUPayment(s.DB, PaymentRow{
		ShopifyOrderID:   strconv.FormatInt(o.ID, 10),
		ShopifyOrderName: o.Name,
		Amount:           amount,
		Currency:         o.Currency,
		CustomerEmail:    o.Email,
		CustomerName:     customerName,
		CustomerPhone:    customerPhone,
	})
	if err != nil {
		return err
	}
	log.Printf("MoneyEU: inserted payment id=%d for order %s", paymentID, o.Name)
	// 2) Create MoneyEU order
	req := CreateOrderExtRequest{
		Amount:          amount,
		Currency:        o.Currency,
		Name:            fallback(customerName, "Customer"),
		Mail:            o.Email,
		PhoneNumber:     customerPhone,
		DialCode:        dialCode,
		Address:         address1,
		City:            city,
		State:           state,
		Zip:             zip,
		Country:         country,
		IdOrderExt:      strconv.FormatInt(o.ID, 10),
		Language:        "English",
		Sms:             false,
		CustomerService: "Lockout Supplements",
	}

	resp, err := s.Client.CreateOrderExt(ctx, req)
	if err != nil {
		return fmt.Errorf("CreateOrderExt: %w", err)
	}

	c, err := resp.FirstContent()
	if err != nil {
		return fmt.Errorf("CreateOrderExt: parse content: %w", err)
	}
	log.Println("MoneyEU: CreateOrderExt response content:", c.ID, c.IdOrderExt, c.Url, c.Status)

	moneyEUOrderID := fmt.Sprintf("%d", c.ID)
	checkoutURL := c.Url
	status := c.Status

	if err := SetMoneyEUOrderLink(s.DB, paymentID, moneyEUOrderID, c.IdOrderExt, checkoutURL, status); err != nil {
		return err
	}

	// 3) Email checkout link
	/*
		subject := fmt.Sprintf("Complete your payment for Order %s", o.Name)
		body := fmt.Sprintf(
			"Hi,\n\nThanks for your order with Lockout Supplements (%s).\n\n"+
				"To complete payment, use the secure checkout link below:\n%s\n\n"+
				"Amount due: %.2f %s\n\n"+
				"If you have any issues, reply to this email and we’ll help.\n\n"+
				"— Lockout Supplements\n",
			o.Name, checkoutURL, amount, o.Currency,
		)

		if err := email.Send(s.SMTP, o.Email, subject, body); err != nil {
			_ = MarkEmailFailed(s.DB, paymentID, err.Error())
			return fmt.Errorf("email send: %w", err)
		}
		_ = MarkEmailSent(s.DB, paymentID)

		log.Printf("MoneyEU: emailed checkout link for order %s", o.Name)
		return nil
	*/
	return nil
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
