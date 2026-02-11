// Package ctasvc chứa service data access cho domain CTA (Call-to-Action).
// Nằm trong folder service/; base service (BaseServiceMongoImpl) ở api/basesvc.
// File: service.cta.library.go - giữ tên cấu trúc cũ (service.<domain>.<entity>.go).
package ctasvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	ctamodels "meta_commerce/internal/api/cta/models"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// CTALibraryService là service quản lý CTA Library (CRUD).
type CTALibraryService struct {
	*basesvc.BaseServiceMongoImpl[ctamodels.CTALibrary]
}

// NewCTALibraryService tạo mới CTALibraryService
func NewCTALibraryService() (*CTALibraryService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CTALibrary)
	if !exist {
		return nil, fmt.Errorf("failed to get cta_library collection: %v", common.ErrNotFound)
	}

	return &CTALibraryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[ctamodels.CTALibrary](collection),
	}, nil
}

// FindOne tìm một CTA Library theo filter (wrapper để package khác gọi được)
func (s *CTALibraryService) FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (ctamodels.CTALibrary, error) {
	return s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
}

// InsertOne tạo mới một CTA Library (wrapper để package khác gọi được)
func (s *CTALibraryService) InsertOne(ctx context.Context, data ctamodels.CTALibrary) (ctamodels.CTALibrary, error) {
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// UpdateOne cập nhật một CTA Library (wrapper để package khác gọi được)
func (s *CTALibraryService) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (ctamodels.CTALibrary, error) {
	return s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, opts)
}
