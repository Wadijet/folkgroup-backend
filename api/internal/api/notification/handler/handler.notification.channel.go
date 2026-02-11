package notifhdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifdto "meta_commerce/internal/api/notification/dto"
	notifsvc "meta_commerce/internal/api/notification/service"
)

// NotificationChannelHandler xử lý các request liên quan đến Notification Channel
type NotificationChannelHandler struct {
	*basehdl.BaseHandler[notifmodels.NotificationChannel, notifdto.NotificationChannelCreateInput, notifdto.NotificationChannelUpdateInput]
	channelService *notifsvc.NotificationChannelService
}

// NewNotificationChannelHandler tạo mới NotificationChannelHandler
func NewNotificationChannelHandler() (*NotificationChannelHandler, error) {
	channelService, err := notifsvc.NewNotificationChannelService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification channel service: %v", err)
	}

	hdl := &NotificationChannelHandler{
		BaseHandler:    basehdl.NewBaseHandler[notifmodels.NotificationChannel, notifdto.NotificationChannelCreateInput, notifdto.NotificationChannelUpdateInput](channelService),
		channelService: channelService,
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
