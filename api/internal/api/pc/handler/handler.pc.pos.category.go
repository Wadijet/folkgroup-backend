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

// PcPosCategoryHandler xử lý các yêu cầu liên quan đến Pancake POS Category
type PcPosCategoryHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput]
	PcPosCategoryService *pcsvc.PcPosCategoryService
}

// NewPcPosCategoryHandler khởi tạo PcPosCategoryHandler mới
func NewPcPosCategoryHandler() (*PcPosCategoryHandler, error) {
	service, err := pcsvc.NewPcPosCategoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos category service: %v", err)
	}
	hdl := &PcPosCategoryHandler{PcPosCategoryService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// SyncUpsertOneFromParts filter + body — logic ở PcPosCategoryService.RunSyncUpsertOneFromJSON.
func (h *PcPosCategoryHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosCategoryService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
