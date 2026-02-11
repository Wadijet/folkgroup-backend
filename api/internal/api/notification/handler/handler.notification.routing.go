package notifhdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifdto "meta_commerce/internal/api/notification/dto"
	notifsvc "meta_commerce/internal/api/notification/service"
)

// NotificationRoutingHandler xử lý các request liên quan đến Notification Routing Rule
type NotificationRoutingHandler struct {
	*basehdl.BaseHandler[notifmodels.NotificationRoutingRule, notifdto.NotificationRoutingRuleCreateInput, notifdto.NotificationRoutingRuleUpdateInput]
	routingService *notifsvc.NotificationRoutingService
}

// NewNotificationRoutingHandler tạo mới NotificationRoutingHandler
func NewNotificationRoutingHandler() (*NotificationRoutingHandler, error) {
	routingService, err := notifsvc.NewNotificationRoutingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification routing service: %v", err)
	}

	hdl := &NotificationRoutingHandler{
		BaseHandler:    basehdl.NewBaseHandler[notifmodels.NotificationRoutingRule, notifdto.NotificationRoutingRuleCreateInput, notifdto.NotificationRoutingRuleUpdateInput](routingService),
		routingService: routingService,
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
