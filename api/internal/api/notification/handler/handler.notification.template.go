package notifhdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifdto "meta_commerce/internal/api/notification/dto"
	notifsvc "meta_commerce/internal/api/notification/service"
)

// NotificationTemplateHandler xử lý các request liên quan đến Notification Template
type NotificationTemplateHandler struct {
	*basehdl.BaseHandler[notifmodels.NotificationTemplate, notifdto.NotificationTemplateCreateInput, notifdto.NotificationTemplateUpdateInput]
}

// NewNotificationTemplateHandler tạo mới NotificationTemplateHandler
func NewNotificationTemplateHandler() (*NotificationTemplateHandler, error) {
	templateService, err := notifsvc.NewNotificationTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification template service: %v", err)
	}

	hdl := &NotificationTemplateHandler{
		BaseHandler: basehdl.NewBaseHandler[notifmodels.NotificationTemplate, notifdto.NotificationTemplateCreateInput, notifdto.NotificationTemplateUpdateInput](templateService),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
