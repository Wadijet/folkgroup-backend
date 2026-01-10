package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// DraftVideoService là service quản lý draft videos (L7)
type DraftVideoService struct {
	*BaseServiceMongoImpl[models.DraftVideo]
}

// NewDraftVideoService tạo mới DraftVideoService
func NewDraftVideoService() (*DraftVideoService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftVideos)
	if !exist {
		return nil, fmt.Errorf("failed to get content_draft_videos collection: %v", common.ErrNotFound)
	}

	return &DraftVideoService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DraftVideo](collection),
	}, nil
}
