package pchdl

import (
	"encoding/json"
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcsvc "meta_commerce/internal/api/pc/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
)

// PcPosWarehouseHandler xử lý các yêu cầu liên quan đến Pancake POS Warehouse
type PcPosWarehouseHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput]
	PcPosWarehouseService *pcsvc.PcPosWarehouseService
}

// NewPcPosWarehouseHandler khởi tạo PcPosWarehouseHandler mới
func NewPcPosWarehouseHandler() (*PcPosWarehouseHandler, error) {
	service, err := pcsvc.NewPcPosWarehouseService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos warehouse service: %v", err)
	}
	hdl := &PcPosWarehouseHandler{PcPosWarehouseService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosWarehouse struct để extract chạy (flatten panCakeData → warehouseId, shopId, name, ...).
func (h *PcPosWarehouseHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var wh pcmodels.PcPosWarehouse
	if err := json.Unmarshal(c.Body(), &wh); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && wh.OwnerOrganizationID.IsZero() {
		wh.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&wh); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosWarehouseService.SyncUpsertOne(c.Context(), filter, &wh)
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
