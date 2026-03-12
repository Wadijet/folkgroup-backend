package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsCampThresholds lưu ngưỡng adaptive theo campaign (P25/P50/P75) từ dữ liệu lịch sử 14 ngày.
// Theo FolkForm v4.1 Section 2.2 — Per-Camp Adaptive Threshold.
// Cập nhật mỗi sáng hoặc khi recalculate campaign.
type AdsCampThresholds struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId          string            `json:"campaignId" bson:"campaignId" index:"single:1,compound:ads_camp_thresholds_lookup"`
	AdAccountId         string            `json:"adAccountId" bson:"adAccountId" index:"single:1,compound:ads_camp_thresholds_lookup"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:ads_camp_thresholds_lookup"`

	// MetaCreatedAt campaign — dùng tính tuổi campaign (giai đoạn 0–3)
	MetaCreatedAt int64 `json:"metaCreatedAt" bson:"metaCreatedAt"`

	// WindowDays số ngày dùng để tính (7 hoặc 14)
	WindowDays int `json:"windowDays" bson:"windowDays"`

	// Percentiles — P25, P50, P75 từ daily metrics 14 ngày
	// CPA Mess (đ): spend/mess mỗi ngày
	CpaMessP25 float64 `json:"cpaMessP25" bson:"cpaMessP25"`
	CpaMessP50 float64 `json:"cpaMessP50" bson:"cpaMessP50"`
	CpaMessP75 float64 `json:"cpaMessP75" bson:"cpaMessP75"`

	// CPA Purchase (đ): spend/orders mỗi ngày
	CpaPurchaseP25 float64 `json:"cpaPurchaseP25" bson:"cpaPurchaseP25"`
	CpaPurchaseP50 float64 `json:"cpaPurchaseP50" bson:"cpaPurchaseP50"`
	CpaPurchaseP75 float64 `json:"cpaPurchaseP75" bson:"cpaPurchaseP75"`

	// Conv Rate (%): orders/mess mỗi ngày
	ConvRateP25 float64 `json:"convRateP25" bson:"convRateP25"`
	ConvRateP50 float64 `json:"convRateP50" bson:"convRateP50"`
	ConvRateP75 float64 `json:"convRateP75" bson:"convRateP75"`

	// CTR (%): từ meta insights
	CtrP25 float64 `json:"ctrP25" bson:"ctrP25"`
	CtrP50 float64 `json:"ctrP50" bson:"ctrP50"`
	CtrP75 float64 `json:"ctrP75" bson:"ctrP75"`

	// AvgDailyMess — số mess trung bình mỗi ngày (dùng cho điều kiện giai đoạn 2: ≥20 mess/ngày)
	AvgDailyMess float64 `json:"avgDailyMess" bson:"avgDailyMess"`

	// DateStart, DateStop — khoảng ngày dùng để tính
	DateStart string `json:"dateStart" bson:"dateStart"`
	DateStop  string `json:"dateStop" bson:"dateStop"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
