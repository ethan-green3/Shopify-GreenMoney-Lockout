package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/burner"
	"Shopify-GreenMoney-Lockout/internal/email"
	"context"
)

type burnerShopifyResolver struct {
	registry *internal.ShopifyClientRegistry
}

func (r burnerShopifyResolver) ForShopDomain(domain string) (burner.ShopifyPayer, error) {
	return r.registry.ForShopDomain(domain)
}

type burnerEmailSender struct{ cfg email.SMTPConfig }

func (s burnerEmailSender) Send(ctx context.Context, to, subject, body string) error {
	return email.SendContext(ctx, s.cfg, to, subject, body)
}
