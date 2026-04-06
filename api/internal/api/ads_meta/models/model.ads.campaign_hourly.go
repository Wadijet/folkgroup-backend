// Package models — Hourly Peak Matrix (FolkForm v4.1 Section 05).
// campaign_hourly: dữ liệu theo giờ mỗi camp (mess, orders, spend, CR).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsCampaignHourly lưu dữ liệu theo giờ của campaign. Thu thập mỗi giờ.
type AdsCampaignHourly struct {
	ID                   primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId           string             `json:"campaignId" bson:"campaignId" index:"single:1,compound:hourly_lookup"`
	AdAccountId          string             `json:"adAccountId" bson:"adAccountId" index:"single:1,compound:hourly_lookup"`
	OwnerOrganizationID  primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:hourly_lookup"`
	Date                 string             `json:"date" bson:"date" index:"single:1,compound:hourly_lookup"` // YYYY-MM-DD
	Hour                 int                `json:"hour" bson:"hour" index:"single:1,compound:hourly_lookup"` // 0-23 (VN)
	Mess                 int64              `json:"mess" bson:"mess"`
	Orders               int64              `json:"orders" bson:"orders"`
	Spend                float64            `json:"spend" bson:"spend"`
	ConvRateHourly       float64            `json:"convRateHourly" bson:"convRateHourly"` // orders/mess khi mess>0
	CreatedAt            int64              `json:"createdAt" bson:"createdAt"`
}
