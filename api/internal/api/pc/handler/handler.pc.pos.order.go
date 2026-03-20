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

// SyncUpsertOneFromParts filter đã ProcessFilter/ProcessMergedFilter + body JSON — logic nghiệp vụ ở PcPosOrderService.RunSyncUpsertOneFromJSON (CIO ingest domain order).
func (h *PcPosOrderHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosOrderService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
