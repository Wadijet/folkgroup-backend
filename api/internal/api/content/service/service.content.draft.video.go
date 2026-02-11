package contentsvc

import (
	"fmt"

	contentmodels "meta_commerce/internal/api/content/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// DraftVideoService là service quản lý draft videos (L7)
type DraftVideoService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.DraftVideo]
}

// NewDraftVideoService tạo mới DraftVideoService
func NewDraftVideoService() (*DraftVideoService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftVideos)
	if !exist {
		return nil, fmt.Errorf("failed to get content_draft_videos collection: %v", common.ErrNotFound)
	}

	return &DraftVideoService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.DraftVideo](collection),
	}, nil
}
