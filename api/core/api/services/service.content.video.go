package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// VideoService là service quản lý videos (L7)
type VideoService struct {
	*BaseServiceMongoImpl[models.Video]
}

// NewVideoService tạo mới VideoService
func NewVideoService() (*VideoService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Videos)
	if !exist {
		return nil, fmt.Errorf("failed to get content_videos collection: %v", common.ErrNotFound)
	}

	return &VideoService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.Video](collection),
	}, nil
}
