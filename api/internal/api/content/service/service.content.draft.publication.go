package contentsvc

import (
	"fmt"

	contentmodels "meta_commerce/internal/api/content/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// DraftPublicationService là service quản lý draft publications (L8)
type DraftPublicationService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.DraftPublication]
}

// NewDraftPublicationService tạo mới DraftPublicationService
func NewDraftPublicationService() (*DraftPublicationService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftPublications)
	if !exist {
		return nil, fmt.Errorf("failed to get content_draft_publications collection: %v", common.ErrNotFound)
	}

	return &DraftPublicationService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.DraftPublication](collection),
	}, nil
}
