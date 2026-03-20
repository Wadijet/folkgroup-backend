package pcsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
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

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosCustomerService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosCustomer, bool, error) {
	var zero pcmodels.PcPosCustomer
	var customer pcmodels.PcPosCustomer
	if err := json.Unmarshal(body, &customer); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && customer.OwnerOrganizationID.IsZero() {
		customer.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&customer); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &customer)
}
