package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// PublicationService là service quản lý publications (L8)
type PublicationService struct {
	*BaseServiceMongoImpl[models.Publication]
}

// NewPublicationService tạo mới PublicationService
func NewPublicationService() (*PublicationService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Publications)
	if !exist {
		return nil, fmt.Errorf("failed to get content_publications collection: %v", common.ErrNotFound)
	}

	return &PublicationService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.Publication](collection),
	}, nil
}
