// Package pcsvc — L1 mirror nhập tay (order_src_manual_*) cùng model Pancake, đồng bộ/upsert giống PcPos* nhưng collection Manual*.
package pcsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// --- Order ---

// ManualPosOrderService service CRUD + sync-upsert trên order_src_manual_orders.
type ManualPosOrderService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosOrder]
}

// NewManualPosOrderService tạo service đơn nhập tay.
func NewManualPosOrderService() (*ManualPosOrderService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosOrders)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos orders collection: %v", common.ErrNotFound)
	}
	return &ManualPosOrderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosOrder](coll),
	}, nil
}

// SyncUpsertOne upsert có so posData.updated_at (posUpdatedAt).
func (s *ManualPosOrderService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosOrder, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

// RunSyncUpsertOneFromJSON CIO / REST sync-upsert đơn nhập tay.
// Filter: bắt buộc ownerOrganizationId; và một trong: orderId (≠0), _id (ObjectId bản ghi), uid (ord_*).
// Body tối thiểu tương tự domain order: posData nếu dùng layout POS; extract flatten + identity 4 lớp trong DoSyncUpsert.
func (s *ManualPosOrderService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosOrder, bool, error) {
	var zero pcmodels.PcPosOrder
	var order pcmodels.PcPosOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && order.OwnerOrganizationID.IsZero() {
		order.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&order); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	filter = buildManualPosOrderSyncUpsertFilter(filter, &order)
	if err := validateManualPosOrderFilter(filter); err != nil {
		return zero, false, err
	}
	return s.SyncUpsertOne(ctx, filter, &order)
}

func buildManualPosOrderSyncUpsertFilter(filter map[string]interface{}, order *pcmodels.PcPosOrder) map[string]interface{} {
	if filter == nil {
		filter = make(map[string]interface{})
	}
	result := make(map[string]interface{})
	for k, v := range filter {
		result[k] = v
	}
	if result["orderId"] == nil && order != nil && order.OrderId != 0 {
		result["orderId"] = order.OrderId
	}
	if result["_id"] == nil && order != nil && !order.ID.IsZero() {
		result["_id"] = order.ID
	}
	if result["uid"] == nil && order != nil && strings.TrimSpace(order.Uid) != "" {
		result["uid"] = strings.TrimSpace(order.Uid)
	}
	if result["ownerOrganizationId"] == nil && order != nil && !order.OwnerOrganizationID.IsZero() {
		result["ownerOrganizationId"] = order.OwnerOrganizationID
	}
	if v := result["orderId"]; v != nil {
		switch x := v.(type) {
		case string:
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				result["orderId"] = n
			}
		case float64:
			result["orderId"] = int64(x)
		case int:
			result["orderId"] = int64(x)
		}
	}
	if v := result["ownerOrganizationId"]; v != nil {
		if s, ok := v.(string); ok && primitive.IsValidObjectID(s) {
			if oid, err := primitive.ObjectIDFromHex(s); err == nil {
				result["ownerOrganizationId"] = oid
			}
		}
	}
	return result
}

func validateManualPosOrderFilter(f map[string]interface{}) error {
	if f == nil || len(f) == 0 {
		return common.NewError(common.ErrCodeValidationFormat, "Filter cần ownerOrganizationId và khóa định vị: orderId, _id hoặc uid", common.StatusBadRequest, nil)
	}
	if f["ownerOrganizationId"] == nil {
		return common.NewError(common.ErrCodeValidationFormat, "Thiếu ownerOrganizationId (JWT hoặc filter)", common.StatusBadRequest, nil)
	}
	hasKey := false
	if v, ok := f["orderId"]; ok {
		switch x := v.(type) {
		case int64:
			if x != 0 {
				hasKey = true
			}
		case int:
			if x != 0 {
				hasKey = true
			}
		case float64:
			if int64(x) != 0 {
				hasKey = true
			}
		}
	}
	if f["_id"] != nil {
		hasKey = true
	}
	if u, ok := f["uid"].(string); ok && strings.TrimSpace(u) != "" {
		hasKey = true
	}
	if !hasKey {
		return common.NewError(common.ErrCodeValidationFormat, "Cần ít nhất một trong: orderId (khác 0), _id, uid (ord_...)", common.StatusBadRequest, nil)
	}
	return nil
}

// --- Product / Variation / Category (posData) ---

// ManualPosProductService mirror nhập tay sản phẩm.
type ManualPosProductService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosProduct] }

func NewManualPosProductService() (*ManualPosProductService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosProducts)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos products: %v", common.ErrNotFound)
	}
	return &ManualPosProductService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosProduct](coll)}, nil
}

func (s *ManualPosProductService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosProduct, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

func (s *ManualPosProductService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosProduct, bool, error) {
	var zero pcmodels.PcPosProduct
	var p pcmodels.PcPosProduct
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}

// ManualPosVariationService mirror biến thể nhập tay.
type ManualPosVariationService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosVariation] }

func NewManualPosVariationService() (*ManualPosVariationService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosVariations)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos variations: %v", common.ErrNotFound)
	}
	return &ManualPosVariationService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosVariation](coll)}, nil
}

func (s *ManualPosVariationService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosVariation, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

func (s *ManualPosVariationService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosVariation, bool, error) {
	var zero pcmodels.PcPosVariation
	var p pcmodels.PcPosVariation
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}

// ManualPosCategoryService mirror danh mục nhập tay.
type ManualPosCategoryService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosCategory] }

func NewManualPosCategoryService() (*ManualPosCategoryService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosCategories)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos categories: %v", common.ErrNotFound)
	}
	return &ManualPosCategoryService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosCategory](coll)}, nil
}

func (s *ManualPosCategoryService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosCategory, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

func (s *ManualPosCategoryService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosCategory, bool, error) {
	var zero pcmodels.PcPosCategory
	var p pcmodels.PcPosCategory
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}

// ManualPosCustomerService mirror khách POS nhập tay.
type ManualPosCustomerService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosCustomer] }

func NewManualPosCustomerService() (*ManualPosCustomerService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosCustomers)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos customers: %v", common.ErrNotFound)
	}
	return &ManualPosCustomerService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosCustomer](coll)}, nil
}

func (s *ManualPosCustomerService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosCustomer, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

func (s *ManualPosCustomerService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosCustomer, bool, error) {
	var zero pcmodels.PcPosCustomer
	var p pcmodels.PcPosCustomer
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}

// ManualPosShopService mirror shop nhập tay (panCakeData).
type ManualPosShopService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosShop] }

func NewManualPosShopService() (*ManualPosShopService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosShops)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos shops: %v", common.ErrNotFound)
	}
	return &ManualPosShopService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosShop](coll)}, nil
}

func (s *ManualPosShopService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosShop, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}

func (s *ManualPosShopService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosShop, bool, error) {
	var zero pcmodels.PcPosShop
	var p pcmodels.PcPosShop
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}

// ManualPosWarehouseService mirror kho nhập tay (panCakeData).
type ManualPosWarehouseService struct{ *basesvc.BaseServiceMongoImpl[pcmodels.PcPosWarehouse] }

func NewManualPosWarehouseService() (*ManualPosWarehouseService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ManualPosWarehouses)
	if !ok {
		return nil, fmt.Errorf("failed to get manual pos warehouses: %v", common.ErrNotFound)
	}
	return &ManualPosWarehouseService{BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosWarehouse](coll)}, nil
}

func (s *ManualPosWarehouseService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosWarehouse, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}

func (s *ManualPosWarehouseService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosWarehouse, bool, error) {
	var zero pcmodels.PcPosWarehouse
	var p pcmodels.PcPosWarehouse
	if err := json.Unmarshal(body, &p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && p.OwnerOrganizationID.IsZero() {
		p.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&p); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &p)
}
