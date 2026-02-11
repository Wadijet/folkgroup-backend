package notifhdl

import (
	"fmt"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	deliverysvc "meta_commerce/internal/api/delivery/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// NotificationHistoryHandler xử lý các request liên quan đến Delivery History (alias cho backward compatibility)
type NotificationHistoryHandler struct {
	*basehdl.BaseHandler[deliverymodels.DeliveryHistory, deliverymodels.DeliveryHistory, deliverymodels.DeliveryHistory]
}

// NewNotificationHistoryHandler tạo mới NotificationHistoryHandler
func NewNotificationHistoryHandler() (*NotificationHistoryHandler, error) {
	historySvc, err := deliverysvc.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification history service: %v", err)
	}
	hdl := &NotificationHistoryHandler{
		BaseHandler: basehdl.NewBaseHandler[deliverymodels.DeliveryHistory, deliverymodels.DeliveryHistory, deliverymodels.DeliveryHistory](historySvc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
