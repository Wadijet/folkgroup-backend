package fbsvc

import (
	"fmt"

	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// FbCustomerService là cấu trúc chứa các phương thức liên quan đến Facebook Customer
type FbCustomerService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbCustomer]
}

// NewFbCustomerService tạo mới FbCustomerService
func NewFbCustomerService() (*FbCustomerService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_customers collection: %v", common.ErrNotFound)
	}
	return &FbCustomerService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbCustomer](coll),
	}, nil
}
