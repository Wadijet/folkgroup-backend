// Package router - Đăng ký route Meta Ads.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	metahdl "meta_commerce/internal/api/meta/handler"
	_ "meta_commerce/internal/api/meta/hooks" // Đăng ký hook tính ads profile (currentMetrics)
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Meta Ads lên v1.
// Tất cả entity Meta Ads có CRUD đầy đủ: AdAccount, Campaign, AdSet, Ad, AdInsight.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	// Meta Token: đổi short-lived → long-lived, lưu vào file
	tokenHandler := metahdl.NewMetaTokenHandler()
	metaTokenExchangeMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/meta/token", "POST", "/exchange", []fiber.Handler{metaTokenExchangeMiddleware}, tokenHandler.HandleExchangeToken)

	orgContextMiddleware := middleware.OrganizationContextMiddleware()

	// Meta Ad Account
	adAccountHandler, err := metahdl.NewMetaAdAccountHandler()
	if err != nil {
		return fmt.Errorf("tạo meta ad account handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/ad-account", adAccountHandler, apirouter.ReadWriteConfig, "MetaAdAccount")
	// Đồng bộ Meta từ agent: POST /api/v1/cio/ingest (domain meta_*)

	// Meta Campaign
	campaignHandler, err := metahdl.NewMetaCampaignHandler()
	if err != nil {
		return fmt.Errorf("tạo meta campaign handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/campaign", campaignHandler, apirouter.ReadWriteConfig, "MetaCampaign")

	// Meta Ad Set
	adSetHandler, err := metahdl.NewMetaAdSetHandler()
	if err != nil {
		return fmt.Errorf("tạo meta ad set handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/ad-set", adSetHandler, apirouter.ReadWriteConfig, "MetaAdSet")

	// Meta Ad
	adHandler, err := metahdl.NewMetaAdHandler()
	if err != nil {
		return fmt.Errorf("tạo meta ad handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/ad", adHandler, apirouter.ReadWriteConfig, "MetaAd")
	apirouter.RegisterRouteWithMiddleware(v1, "/meta/ad", "POST", "/recalculate", []fiber.Handler{middleware.AuthMiddleware("MetaAd.Update"), orgContextMiddleware}, adHandler.HandleRecalculate)
	apirouter.RegisterRouteWithMiddleware(v1, "/meta/ad", "POST", "/recalculate-all", []fiber.Handler{middleware.AuthMiddleware("MetaAd.Update"), orgContextMiddleware}, adHandler.HandleRecalculateAllMetaAds)

	// Meta Ad Insight
	adInsightHandler, err := metahdl.NewMetaAdInsightHandler()
	if err != nil {
		return fmt.Errorf("tạo meta ad insight handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/ad-insight", adInsightHandler, apirouter.ReadWriteConfig, "MetaAdInsight")

	// Meta Activity History — lịch sử thay đổi metrics (Campaign/AdSet/Ad). Chỉ đọc.
	activityHistoryHandler, err := metahdl.NewMetaAdsActivityHistoryHandler()
	if err != nil {
		return fmt.Errorf("tạo meta activity history handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/meta/activity-history", activityHistoryHandler, apirouter.ReadOnlyConfig, "MetaActivityHistory")

	return nil
}
