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

// PcPosVariationHandler xử lý các yêu cầu liên quan đến Pancake POS Variation
type PcPosVariationHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput]
	PcPosVariationService *pcsvc.PcPosVariationService
}

// NewPcPosVariationHandler khởi tạo PcPosVariationHandler mới
func NewPcPosVariationHandler() (*PcPosVariationHandler, error) {
	service, err := pcsvc.NewPcPosVariationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos variation service: %v", err)
	}
	hdl := &PcPosVariationHandler{PcPosVariationService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// SyncUpsertOneFromParts filter + body — logic ở PcPosVariationService.RunSyncUpsertOneFromJSON.
func (h *PcPosVariationHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosVariationService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
