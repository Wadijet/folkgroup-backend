package contentsvc

import (
	"fmt"

	contentmodels "meta_commerce/internal/api/content/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PublicationService là service quản lý publications (L8)
type PublicationService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.Publication]
}

// NewPublicationService tạo mới PublicationService
func NewPublicationService() (*PublicationService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Publications)
	if !exist {
		return nil, fmt.Errorf("failed to get content_publications collection: %v", common.ErrNotFound)
	}

	return &PublicationService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.Publication](collection),
	}, nil
}
