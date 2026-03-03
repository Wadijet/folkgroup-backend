package pchdl

import (
	"encoding/json"
	"fmt"
	"strconv"

	basehdl "meta_commerce/internal/api/base/handler"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcsvc "meta_commerce/internal/api/pc/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PcPosOrderHandler xử lý các yêu cầu liên quan đến Pancake POS Order
type PcPosOrderHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput]
	PcPosOrderService *pcsvc.PcPosOrderService
}

// NewPcPosOrderHandler khởi tạo PcPosOrderHandler mới
func NewPcPosOrderHandler() (*PcPosOrderHandler, error) {
	service, err := pcsvc.NewPcPosOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos order service: %v", err)
	}
	hdl := &PcPosOrderHandler{PcPosOrderService: service}
	// Dùng full service để CRUD đi qua BaseServiceMongoImpl (đã tích hợp EmitDataChanged)
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput](service)
	return hdl, nil
}

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosOrder struct để extract chạy (flatten posData → orderId, shopId, status, ...).
// Filter phải có orderId và ownerOrganizationId để tránh duplicate. Nếu thiếu, bổ sung từ body/struct.
func (h *PcPosOrderHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var order pcmodels.PcPosOrder
	if err := json.Unmarshal(c.Body(), &order); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && order.OwnerOrganizationID.IsZero() {
		order.OwnerOrganizationID = *orgID
	}
	// Extract posData → orderId, shopId, status, posUpdatedAt, ... (để filter và $set đủ field)
	if err := utility.ExtractDataIfExists(&order); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	// Bổ sung filter từ struct nếu thiếu orderId hoặc ownerOrganizationId (tránh duplicate)
	filter = h.buildSyncUpsertFilter(filter, &order)
	if filter["orderId"] == nil || filter["ownerOrganizationId"] == nil {
		return c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code":    common.ErrCodeValidationFormat.Code,
			"message": "Filter hoặc body phải có orderId và ownerOrganizationId để sync-upsert đơn hàng",
			"status":  "error",
		})
	}
	result, skipped, err := h.PcPosOrderService.SyncUpsertOne(c.Context(), filter, &order)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// buildSyncUpsertFilter bổ sung filter từ struct khi thiếu orderId/ownerOrganizationId, và chuẩn hóa orderId (string→int64).
// Tránh duplicate do filter không nhất quán (vd: orderId string "123" vs int 123).
func (h *PcPosOrderHandler) buildSyncUpsertFilter(filter map[string]interface{}, order *pcmodels.PcPosOrder) map[string]interface{} {
	if filter == nil {
		filter = make(map[string]interface{})
	}
	result := make(map[string]interface{})
	for k, v := range filter {
		result[k] = v
	}
	// Bổ sung orderId từ struct (đã extract từ posData) nếu thiếu
	if result["orderId"] == nil && order != nil && order.OrderId != 0 {
		result["orderId"] = order.OrderId
	}
	// Bổ sung ownerOrganizationId từ struct nếu thiếu
	if result["ownerOrganizationId"] == nil && order != nil && !order.OwnerOrganizationID.IsZero() {
		result["ownerOrganizationId"] = order.OwnerOrganizationID
	}
	// Chuẩn hóa orderId từ query: string "123" -> int64 123 (tránh duplicate do type mismatch)
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
	// Chuẩn hóa ownerOrganizationId: string hex -> ObjectID
	if v := result["ownerOrganizationId"]; v != nil {
		if s, ok := v.(string); ok && primitive.IsValidObjectID(s) {
			if oid, err := primitive.ObjectIDFromHex(s); err == nil {
				result["ownerOrganizationId"] = oid
			}
		}
	}
	return result
}
