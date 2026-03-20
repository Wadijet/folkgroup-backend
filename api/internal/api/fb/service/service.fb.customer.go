package fbsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	basesvc "meta_commerce/internal/api/base/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
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

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *FbCustomerService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (fbmodels.FbCustomer, bool, error) {
	var zero fbmodels.FbCustomer
	var customer fbmodels.FbCustomer
	if err := json.Unmarshal(body, &customer); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && customer.OwnerOrganizationID.IsZero() {
		customer.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&customer); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &customer)
}
