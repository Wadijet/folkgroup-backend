package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosCustomerService là cấu trúc chứa các phương thức liên quan đến Pancake POS Customer
type PcPosCustomerService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosCustomer]
}

// NewPcPosCustomerService tạo mới PcPosCustomerService
func NewPcPosCustomerService() (*PcPosCustomerService, error) {
	pcPosCustomerCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_customers collection: %v", common.ErrNotFound)
	}

	return &PcPosCustomerService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosCustomer](pcPosCustomerCollection),
	}, nil
}
