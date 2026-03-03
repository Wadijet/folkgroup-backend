package pcsvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
// Dùng chung logic với Upsert; khác biệt duy nhất là so sánh updated_at.
func (s *PcPosCustomerService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosCustomer, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}
