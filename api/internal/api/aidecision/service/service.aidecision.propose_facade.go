// Package aidecisionsvc — Facade Propose cho Vision 08 Phase 0.
// Chỉ AI Decision gọi Propose; domain modules (Ads, CIO) gọi qua facade này.
package aidecisionsvc

import (
	"context"

	"meta_commerce/internal/approval"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProposeForAds tạo đề xuất ads qua Executor (Vision 08: Ads gọi qua AI Decision).
// Nhận input đã build sẵn từ Ads, forward sang approval.Propose(domain=ads).
func ProposeForAds(ctx context.Context, input approval.ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (*approval.ActionPending, error) {
	return approval.Propose(ctx, "ads", input, ownerOrgID, baseURL)
}

// ProposeAndApproveForAds tạo đề xuất ads và approve ngay (cho action auto).
// Vision 08: Ads gọi qua AI Decision khi cần ProposeAndApproveAuto.
func ProposeAndApproveForAds(ctx context.Context, input approval.ProposeInput, ownerOrgID primitive.ObjectID) (*approval.ActionPending, error) {
	return approval.ProposeAndApproveAuto(ctx, "ads", input, ownerOrgID)
}
