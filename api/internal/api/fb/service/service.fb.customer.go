package fbsvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (panCakeUpdatedAt) hoặc document chưa tồn tại.
// Dùng chung logic với Upsert; khác biệt duy nhất là so sánh updated_at.
func (s *FbCustomerService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (fbmodels.FbCustomer, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}
