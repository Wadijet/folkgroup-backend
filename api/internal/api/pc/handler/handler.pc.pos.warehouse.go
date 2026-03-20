package pchdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcsvc "meta_commerce/internal/api/pc/service"
	"meta_commerce/internal/common"

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

// SyncUpsertOneFromParts filter + body — logic ở PcPosWarehouseService.RunSyncUpsertOneFromJSON.
func (h *PcPosWarehouseHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosWarehouseService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
