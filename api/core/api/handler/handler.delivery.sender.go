package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// DeliverySenderHandler xử lý các request liên quan đến Notification Sender (Hệ thống 1)
// Alias của NotificationSenderHandler để dùng trong delivery namespace
type DeliverySenderHandler struct {
	BaseHandler[models.NotificationChannelSender, dto.NotificationChannelSenderCreateInput, dto.NotificationChannelSenderUpdateInput]
}

// NewDeliverySenderHandler tạo mới DeliverySenderHandler
func NewDeliverySenderHandler() (*DeliverySenderHandler, error) {
	senderService, err := services.NewNotificationSenderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification sender service: %v", err)
	}

	baseHandler := NewBaseHandler[models.NotificationChannelSender, dto.NotificationChannelSenderCreateInput, dto.NotificationChannelSenderUpdateInput](senderService)
	handler := &DeliverySenderHandler{
		BaseHandler: *baseHandler,
	}

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{
			"smtpPassword",
			"botToken",
		},
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
