package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaAdInsightDailySnapshot lưu snapshot cumulative "today" mỗi lần agent sync insights (15p/lần).
// So sánh snapshot mới vs cũ → suy ra spend/impressions theo từng giờ (hoặc 30p).
// Dùng cho CB-1 (spend_30p, spend_yesterday_cùng_30p), CB-2 (CPM_30p), CB-3 (spend=0).
//
// Ví dụ: sync 09:00 spend=1tr, sync 09:30 spend=1.2tr → spend_09:00_09:30 = 200k.
// spend_09:00_10:00 = snapshot_10:00.spend - snapshot_09:00.spend.
type MetaAdInsightDailySnapshot struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ObjectId            string             `json:"objectId" bson:"objectId" index:"single:1,compound:snapshot_lookup"`
	ObjectType          string             `json:"objectType" bson:"objectType" index:"single:1,compound:snapshot_lookup"`
	AdAccountId         string             `json:"adAccountId" bson:"adAccountId" index:"single:1,compound:snapshot_lookup"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:snapshot_lookup"`
	Date                string             `json:"date" bson:"date" index:"single:1,compound:snapshot_lookup"` // YYYY-MM-DD (từ dateStart của insight)
	SnapshotAt          int64              `json:"snapshotAt" bson:"snapshotAt" index:"single:1,compound:snapshot_lookup"` // Unix ms — thời điểm snapshot

	// Metrics cumulative từ 00:00 đến SnapshotAt (giá trị từ Meta API cho date_preset=today)
	Spend       float64 `json:"spend" bson:"spend"`
	Impressions int64   `json:"impressions" bson:"impressions"`
	Clicks      int64   `json:"clicks" bson:"clicks"`
	Reach       int64   `json:"reach" bson:"reach"`
	Cpm         float64 `json:"cpm" bson:"cpm"`
	Ctr         float64 `json:"ctr" bson:"ctr"`
	Cpc         float64 `json:"cpc" bson:"cpc"`

	CreatedAt int64     `json:"createdAt" bson:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt" bson:"expiresAt" index:"single:1,ttl:0"` // TTL: xóa khi ExpiresAt đã qua (set = now + 7 ngày)
}
