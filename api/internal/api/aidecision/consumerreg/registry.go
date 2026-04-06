// Package consumerreg — Đăng ký handler consumer theo eventType (chuẩn hoá dần: tách map khỏi worker, cho phép đăng ký từ init module khác sau này).
package consumerreg

import (
	"context"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

// Handler xử lý một DecisionEvent sau intake (cùng chữ ký worker consumer).
type Handler func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error

var handlers = make(map[string]Handler)

// Register gắn eventType → handler (ghi đè nếu đăng ký lại — giữ tương thích hành vi init cũ).
func Register(eventType string, h Handler) {
	if eventType == "" || h == nil {
		return
	}
	handlers[eventType] = h
}

// RegisterMany gắn cùng một handler cho nhiều eventType.
func RegisterMany(types []string, h Handler) {
	for _, t := range types {
		Register(t, h)
	}
}

// Lookup trả handler đã đăng ký.
func Lookup(eventType string) (Handler, bool) {
	h, ok := handlers[eventType]
	return h, ok && h != nil
}
