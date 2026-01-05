package handler

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// NotificationHistoryHandler xử lý các request liên quan đến Delivery History (alias cho backward compatibility)
// Note: Thực chất sử dụng DeliveryHistory model (thuộc Delivery System)
type NotificationHistoryHandler struct {
	BaseHandler[models.DeliveryHistory, models.DeliveryHistory, models.DeliveryHistory]
}

// NewNotificationHistoryHandler tạo mới NotificationHistoryHandler
func NewNotificationHistoryHandler() (*NotificationHistoryHandler, error) {
	historyService, err := services.NewDeliveryHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification history service: %v", err)
	}

	handler := &NotificationHistoryHandler{}
	handler.BaseService = historyService

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{},
		AllowedOperators: []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		},
		MaxFields: 10,
	}

	return handler, nil
}

