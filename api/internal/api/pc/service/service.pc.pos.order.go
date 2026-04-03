package pcsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
	"meta_commerce/internal/utility/identity"
)

// PcPosOrderService là cấu trúc chứa các phương thức liên quan đến Pancake POS Order.
// Report MarkDirty được xử lý qua event OnDataChanged (package report), không cần override CRUD.
type PcPosOrderService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosOrder]
}

// NewPcPosOrderService tạo mới PcPosOrderService.
func NewPcPosOrderService() (*PcPosOrderService, error) {
	orderCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_orders collection: %v", common.ErrNotFound)
	}
	return &PcPosOrderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosOrder](orderCollection),
	}, nil
}

// SyncFlattenedFromPosData đọc document theo id, chạy extract từ posData vào các field flatten (billFullName, status, posCreatedAt, ...) rồi ghi lại document.
// Dùng để sửa document cũ thiếu field flatten (ví dụ do webhook cũ hoặc extract lỗi trước khi có unwrap Extended JSON).
// ReplaceOne không đi qua BaseServiceMongoImpl nên cần phát event thủ công.
func (s *PcPosOrderService) SyncFlattenedFromPosData(ctx context.Context, id primitive.ObjectID) (pcmodels.PcPosOrder, error) {
	var zero pcmodels.PcPosOrder
	order, err := s.BaseServiceMongoImpl.FindOneById(ctx, id)
	if err != nil {
		return zero, err
	}
	if len(order.PosData) == 0 {
		return zero, fmt.Errorf("document không có posData để sync")
	}
	prevOrder := order // Lưu bản cũ trước khi mutate để truyền PreviousDocument
	if err := utility.ExtractDataIfExists(&order); err != nil {
		return zero, fmt.Errorf("extract từ posData thất bại: %w", err)
	}
	order.UpdatedAt = time.Now().UnixMilli()
	dataMap, err := utility.ToMap(&order)
	if err != nil {
		return zero, fmt.Errorf("ToMap thất bại: %w", err)
	}
	if identity.ShouldEnrich(global.MongoDB_ColNames.PcPosOrders) {
		if err := identity.EnrichIdentity4Layers(ctx, global.MongoDB_ColNames.PcPosOrders, dataMap, nil); err != nil {
			return zero, fmt.Errorf("enrich identity 4 lớp trước ReplaceOne thất bại: %w", err)
		}
	}
	_, err = s.Collection().ReplaceOne(ctx, bson.M{"_id": id}, dataMap)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}
	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName:   global.MongoDB_ColNames.PcPosOrders,
		Operation:        events.OpUpdate,
		Document:         order,
		PreviousDocument: prevOrder,
	})
	return order, nil
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
// Dùng chung logic với Upsert; khác biệt duy nhất là so sánh updated_at.
func (s *PcPosOrderService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosOrder, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

// RunSyncUpsertOneFromJSON gom logic sync-upsert từ JSON body + filter.
//
// CIO POST /v1/cio/ingest — domain "order": filter khuyến nghị { "orderId", "shopId"? };
// ownerOrganizationId lấy từ JWT (active org) nếu body/filter chưa có. Body tối thiểu { "posData": <object POS> };
// extract flatten + identity 4 lớp chạy trong DoSyncUpsert.
func (s *PcPosOrderService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosOrder, bool, error) {
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
	filter = buildPcPosOrderSyncUpsertFilter(filter, &order)
	if filter["orderId"] == nil || filter["ownerOrganizationId"] == nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Filter hoặc body phải có orderId và ownerOrganizationId để sync-upsert đơn hàng", common.StatusBadRequest, nil)
	}
	return s.SyncUpsertOne(ctx, filter, &order)
}

func buildPcPosOrderSyncUpsertFilter(filter map[string]interface{}, order *pcmodels.PcPosOrder) map[string]interface{} {
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
