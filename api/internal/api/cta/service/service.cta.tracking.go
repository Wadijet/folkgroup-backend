// Package ctasvc - CTA Tracking service (xem service.cta.library.go cho package doc).
// File: service.cta.tracking.go - giữ tên cấu trúc cũ (service.<domain>.<entity>.go).
package ctasvc

import (
	"context"
	"fmt"

	ctamodels "meta_commerce/internal/api/cta/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// CTATrackingService là service quản lý CTA Tracking
type CTATrackingService struct {
	*basesvc.BaseServiceMongoImpl[ctamodels.CTATracking]
}

// NewCTATrackingService tạo mới CTATrackingService
func NewCTATrackingService() (*CTATrackingService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CTATracking)
	if !exist {
		return nil, fmt.Errorf("failed to get cta_tracking collection: %v", common.ErrNotFound)
	}

	return &CTATrackingService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[ctamodels.CTATracking](collection),
	}, nil
}

// InsertOne tạo mới một CTA Tracking (wrapper để package khác gọi được)
func (s *CTATrackingService) InsertOne(ctx context.Context, data ctamodels.CTATracking) (ctamodels.CTATracking, error) {
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
