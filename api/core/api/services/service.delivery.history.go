package services

import (
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// DeliveryHistoryService là cấu trúc chứa các phương thức liên quan đến Delivery History (thuộc Delivery System)
type DeliveryHistoryService struct {
	*BaseServiceMongoImpl[models.DeliveryHistory]
}

// NewDeliveryHistoryService tạo mới DeliveryHistoryService
func NewDeliveryHistoryService() (*DeliveryHistoryService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryHistory)
	if !exist {
		return nil, fmt.Errorf("failed to get delivery_history collection: %v", common.ErrNotFound)
	}

	return &DeliveryHistoryService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DeliveryHistory](collection),
	}, nil
}
