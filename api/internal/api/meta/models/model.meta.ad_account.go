package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaAdAccount lưu thông tin Ad Account từ Meta Marketing API (act_xxx).
// Mỗi organization có thể có một hoặc nhiều ad accounts.
// Các field AdAccountId, Name, AccountStatus, Currency có extract tag để lấy từ metaData khi đọc.
type MetaAdAccount struct {
	ID                   primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	AdAccountId          string                 `json:"adAccountId" bson:"adAccountId" index:"unique;text" extract:"metaData\\.id,converter=string,optional"`           // act_123456789 (extract từ metaData["id"])
	Name                 string                 `json:"name" bson:"name" index:"text" extract:"metaData\\.name,converter=string,optional"`                              // Tên ad account (extract từ metaData["name"])
	AccountStatus        int64                  `json:"accountStatus" bson:"accountStatus" extract:"metaData\\.account_status,converter=int64,optional"`              // Trạng thái 1=active, 2=disabled (extract từ metaData["account_status"])
	Currency             string                 `json:"currency" bson:"currency" extract:"metaData\\.currency,converter=string,optional"`                             // Đơn vị tiền tệ VND, USD (extract từ metaData["currency"])
	MetaData             map[string]interface{} `json:"metaData" bson:"metaData"`                                                                                     // Dữ liệu gốc từ Meta API
	OwnerOrganizationID  primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	CreatedAt            int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64                  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt         int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"` // Lần sync cuối

	// AccountMode: config chiến lược (Layer 4 — Account Mode System). BLITZ | NORMAL | EFFICIENCY | PROTECT.
	// Do người dùng/hệ thống đặt, không tính từ metrics. Xem ADS_INTELLIGENCE_DESIGN.md.
	AccountMode string `json:"accountMode,omitempty" bson:"accountMode,omitempty"`

	// CurrentMetrics: trạng thái metrics hiện tại (raw/layer1/layer2/layer3). Cập nhật khi insight sync hoặc order mới.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`
}
