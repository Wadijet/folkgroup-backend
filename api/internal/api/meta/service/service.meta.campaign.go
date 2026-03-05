// Package metasvc - Service quản lý meta campaigns.
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaCampaignService service quản lý meta campaigns.
// Kế thừa BaseServiceMongoImpl, dùng Upsert từ base (filter campaignId).
type MetaCampaignService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.MetaCampaign]
}

// NewMetaCampaignService tạo MetaCampaignService.
func NewMetaCampaignService() (*MetaCampaignService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	return &MetaCampaignService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.MetaCampaign](coll),
	}, nil
}
