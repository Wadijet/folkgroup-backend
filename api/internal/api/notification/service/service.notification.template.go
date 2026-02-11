package notifsvc

import (
	"fmt"

	notifmodels "meta_commerce/internal/api/notification/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// NotificationTemplateService là cấu trúc chứa các phương thức liên quan đến Notification Template
type NotificationTemplateService struct {
	*basesvc.BaseServiceMongoImpl[notifmodels.NotificationTemplate]
}

// NewNotificationTemplateService tạo mới NotificationTemplateService
func NewNotificationTemplateService() (*NotificationTemplateService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationTemplates)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_templates collection: %v", common.ErrNotFound)
	}

	return &NotificationTemplateService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[notifmodels.NotificationTemplate](collection),
	}, nil
}
