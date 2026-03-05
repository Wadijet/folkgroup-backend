// Package dto - DTO cho Meta token exchange.
package dto

// MetaTokenExchangeInput body cho POST /meta/token/exchange.
// shortLivedToken: token ngắn hạn từ Meta Login (Graph API Explorer, Facebook Login flow).
type MetaTokenExchangeInput struct {
	ShortLivedToken string `json:"shortLivedToken"` // Token ngắn hạn cần đổi sang dài hạn (~60 ngày)
}
