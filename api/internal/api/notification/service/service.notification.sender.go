package notifsvc

import (
	"fmt"

	notifmodels "meta_commerce/internal/api/notification/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// NotificationSenderService là cấu trúc chứa các phương thức liên quan đến Notification Sender
type NotificationSenderService struct {
	*basesvc.BaseServiceMongoImpl[notifmodels.NotificationChannelSender]
}

// NewNotificationSenderService tạo mới NotificationSenderService
func NewNotificationSenderService() (*NotificationSenderService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationSenders)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_senders collection: %v", common.ErrNotFound)
	}

	return &NotificationSenderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[notifmodels.NotificationChannelSender](collection),
	}, nil
}
