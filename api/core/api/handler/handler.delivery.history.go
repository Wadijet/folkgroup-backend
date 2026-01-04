package handler

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// DeliveryHistoryHandler xử lý các request liên quan đến Notification History (Hệ thống 1)
// Alias của NotificationHistoryHandler để dùng trong delivery namespace
type DeliveryHistoryHandler struct {
	BaseHandler[models.NotificationHistory, models.NotificationHistory, models.NotificationHistory]
}

// NewDeliveryHistoryHandler tạo mới DeliveryHistoryHandler
func NewDeliveryHistoryHandler() (*DeliveryHistoryHandler, error) {
	historyService, err := services.NewNotificationHistoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification history service: %v", err)
	}

	handler := &DeliveryHistoryHandler{}
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
