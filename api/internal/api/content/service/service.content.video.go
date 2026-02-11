package contentsvc

import (
	"fmt"

	contentmodels "meta_commerce/internal/api/content/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// VideoService là service quản lý videos (L7)
type VideoService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.Video]
}

// NewVideoService tạo mới VideoService
func NewVideoService() (*VideoService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Videos)
	if !exist {
		return nil, fmt.Errorf("failed to get content_videos collection: %v", common.ErrNotFound)
	}

	return &VideoService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.Video](collection),
	}, nil
}
