package notifhdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifdto "meta_commerce/internal/api/notification/dto"
	notifsvc "meta_commerce/internal/api/notification/service"
)

// NotificationSenderHandler xử lý các request liên quan đến Notification Sender
type NotificationSenderHandler struct {
	*basehdl.BaseHandler[notifmodels.NotificationChannelSender, notifdto.NotificationChannelSenderCreateInput, notifdto.NotificationChannelSenderUpdateInput]
}

// NewNotificationSenderHandler tạo mới NotificationSenderHandler
func NewNotificationSenderHandler() (*NotificationSenderHandler, error) {
	senderService, err := notifsvc.NewNotificationSenderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification sender service: %v", err)
	}

	hdl := &NotificationSenderHandler{
		BaseHandler: basehdl.NewBaseHandler[notifmodels.NotificationChannelSender, notifdto.NotificationChannelSenderCreateInput, notifdto.NotificationChannelSenderUpdateInput](senderService),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields: []string{"smtpPassword", "botToken"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields: 10,
	})
	return hdl, nil
}
