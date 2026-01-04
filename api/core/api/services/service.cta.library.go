package services

import (
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// CTALibraryService là service quản lý CTA Library
type CTALibraryService struct {
	*BaseServiceMongoImpl[models.CTALibrary]
}

// NewCTALibraryService tạo mới CTALibraryService
func NewCTALibraryService() (*CTALibraryService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CTALibrary)
	if !exist {
		return nil, fmt.Errorf("failed to get cta_library collection: %v", common.ErrNotFound)
	}

	return &CTALibraryService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.CTALibrary](collection),
	}, nil
}
