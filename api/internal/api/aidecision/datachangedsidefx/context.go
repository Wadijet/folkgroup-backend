// Package datachangedsidefx — Đăng ký side-effect sau datachanged (một cửa gọi từ worker AID).
// Mỗi miền gọi Register trong init; worker build ApplyContext rồi Run.
package datachangedsidefx

import (
	"context"
	"time"

	"meta_commerce/internal/api/aidecision/eventintake"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/aidecision/routecontract"
	"meta_commerce/internal/api/events"
)

// ApplyContext — trạng thái chung sau policy + cửa sổ defer (worker tính trước khi Run).
// Không tham chiếu *aidecisionsvc.AIDecisionService ở đây để tránh vòng import (worker → crm/datachanged → datachangedsidefx → service).
type ApplyContext struct {
	Ctx    context.Context
	Evt    *aidecisionmodels.DecisionEvent
	E      events.DataChangeEvent
	Src    string
	Op     string
	IDHex  string
	OrgHex string

	Dec eventintake.SideEffectDecision

	// Route — định tuyến hiệu lực (code + YAML); contributor kiểm tra cờ pipeline trước khi chạy.
	Route routecontract.Decision

	IngestWin  time.Duration
	ReportWin  time.Duration
	RefreshWin time.Duration

	// CixIntelDefer — chỉ áp khi Src là fb_message_items (worker đã tính).
	CixIntelDefer time.Duration
	// OrderIntelDefer — chỉ áp khi Src là pc_pos_orders (worker đã tính).
	OrderIntelDefer time.Duration
}
