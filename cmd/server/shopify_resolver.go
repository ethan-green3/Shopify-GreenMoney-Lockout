package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/moneyeu"
)

type moneyeuShopifyResolver struct {
	registry *internal.ShopifyClientRegistry
}

func (r moneyeuShopifyResolver) ForShopDomain(shopDomain string) (moneyeu.ShopifyPayer, error) {
	return r.registry.ForShopDomain(shopDomain)
}
