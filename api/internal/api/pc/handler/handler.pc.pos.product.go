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

// PcPosProductHandler xử lý các yêu cầu liên quan đến Pancake POS Product
type PcPosProductHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosProduct, pcdto.PcPosProductCreateInput, pcdto.PcPosProductCreateInput]
	PcPosProductService *pcsvc.PcPosProductService
}

// NewPcPosProductHandler khởi tạo PcPosProductHandler mới
func NewPcPosProductHandler() (*PcPosProductHandler, error) {
	service, err := pcsvc.NewPcPosProductService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos product service: %v", err)
	}
	hdl := &PcPosProductHandler{PcPosProductService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosProduct, pcdto.PcPosProductCreateInput, pcdto.PcPosProductCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// SyncUpsertOneFromParts filter + body — logic ở PcPosProductService.RunSyncUpsertOneFromJSON.
func (h *PcPosProductHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosProductService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
