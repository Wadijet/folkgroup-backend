package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// DraftPublicationService là service quản lý draft publications (L8)
type DraftPublicationService struct {
	*BaseServiceMongoImpl[models.DraftPublication]
}

// NewDraftPublicationService tạo mới DraftPublicationService
func NewDraftPublicationService() (*DraftPublicationService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftPublications)
	if !exist {
		return nil, fmt.Errorf("failed to get content_draft_publications collection: %v", common.ErrNotFound)
	}

	return &DraftPublicationService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DraftPublication](collection),
	}, nil
}
