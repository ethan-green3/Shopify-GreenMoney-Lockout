package internal

import (
	"log/slog"
	"strconv"

	"github.com/lmittmann/tint"
)

func logShopifyOrderMarkedPaid(processor string, shopDomain string, shopifyOrderID int64) {
	slog.Info(
		"Shopify order marked paid",
		tint.Attr(10, slog.String("status", "paid")),
		slog.String("processor", processor),
		slog.String("shop_domain", shopDomain),
		slog.String("shopify_order_id", strconv.FormatInt(shopifyOrderID, 10)),
	)
}
