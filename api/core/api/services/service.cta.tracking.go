package services

import (
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// CTATrackingService là service quản lý CTA Tracking
type CTATrackingService struct {
	*BaseServiceMongoImpl[models.CTATracking]
}

// NewCTATrackingService tạo mới CTATrackingService
func NewCTATrackingService() (*CTATrackingService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CTATracking)
	if !exist {
		return nil, fmt.Errorf("failed to get cta_tracking collection: %v", common.ErrNotFound)
	}

	return &CTATrackingService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.CTATracking](collection),
	}, nil
}
