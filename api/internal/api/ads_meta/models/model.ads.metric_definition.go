// Package models — Định nghĩa metric cho Ads evaluation (FolkForm v4.1).
// Mỗi metric có window (7d, 2h, 1h, 30p), nguồn dữ liệu, công thức.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Các window chuẩn theo FolkForm v4.1.
const (
	Window7d  = "7d"  // 7 ngày — MQS, Mess Trap, Kill rules, currentMetrics
	Window2h  = "2h"  // 2 giờ — Momentum Tracker, CR_now, CB-4
	Window1h  = "1h"  // 1 giờ — CB-1 ROAS, HB-3 Divergence
	Window30p = "30p" // 30 phút — Msg_Rate, early warning
	Window2d  = "2d"  // 2 ngày — Purchase_2day_rolling
	Window3d  = "3d"  // 3 ngày — CTR_3day_avg, CPM_3day_avg
	Window14d = "14d" // 14 ngày — Adaptive threshold
)

// Các loại metric.
const (
	MetricTypeRaw     = "raw"     // Lấy trực tiếp từ collection
	MetricTypeDerived = "derived" // Tính từ raw metrics
)

// Các nguồn dữ liệu.
const (
	SourceMeta             = "meta"              // meta_ad_insights
	SourcePancakePos       = "pancake.pos"       // order_canonical (đồng bộ từ POS)
	SourcePancakeConversation = "pancake.conversation" // fb_conversations
	SourceDerived          = "derived"           // Tính từ raw
)

// AdsMetricDefinition định nghĩa đầy đủ một metric — window, nguồn, công thức.
// Key là unique: có thể là "orders_7d" (key+window) hoặc "spend" (khi chỉ có 1 window).
// Theo FolkForm v4.1: 7d cho Kill/MQS, 2h cho Momentum Tracker, 1h cho CB/HB.
type AdsMetricDefinition struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Key         string             `json:"key" bson:"key" index:"unique:1"`                   // Mã duy nhất: spend, orders_7d, orders_2h, convRate_7d, ...
	Label       string             `json:"label" bson:"label"`                                // Label hiển thị
	Description string             `json:"description" bson:"description"`                    // Mô tả / công thức
	Unit        string             `json:"unit" bson:"unit"`                                  // VND, %, số, phút
	Source      string             `json:"source" bson:"source"`                              // meta, pancake.pos, pancake.conversation, derived
	Window      string             `json:"window" bson:"window"`                               // 7d, 2h, 1h, 30p, 2d, 3d, 14d
	WindowMs    int64              `json:"windowMs" bson:"windowMs"`                          // Số ms tương ứng (để tính nhanh)
	Type        string             `json:"type" bson:"type"`                                   // raw | derived
	FormulaRef  string             `json:"formulaRef,omitempty" bson:"formulaRef,omitempty"`   // derived: "orders/mess", "spend/mess"
	DependsOn   []string           `json:"dependsOn,omitempty" bson:"dependsOn,omitempty"`     // derived: ["orders_7d", "mess_7d"]
	SourceCollection string        `json:"sourceCollection,omitempty" bson:"sourceCollection,omitempty"` // raw: meta_ad_insights, order_canonical
	TimeField   string             `json:"timeField,omitempty" bson:"timeField,omitempty"`     // raw: dateStart, posCreatedAt, panCakeUpdatedAt
	AggregationField string        `json:"aggregationField,omitempty" bson:"aggregationField,omitempty"` // raw: posData.ad_id, panCakeData.ad_ids
	OutputPath  string             `json:"outputPath,omitempty" bson:"outputPath,omitempty"`   // Đường dẫn trong raw: meta.spend, pancake.pos.orders
	UseCase     string             `json:"useCase,omitempty" bson:"useCase,omitempty"`          // MQS, Momentum Tracker, CB-4, ...
	DocReference string            `json:"docReference,omitempty" bson:"docReference,omitempty"` // Tham chiếu FolkForm doc
	Order       int                `json:"order" bson:"order"`                                 // Thứ tự hiển thị
	IsActive    bool               `json:"isActive" bson:"isActive"`                          // Bật/tắt
	CreatedAt   int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt   int64              `json:"updatedAt" bson:"updatedAt"`
}
