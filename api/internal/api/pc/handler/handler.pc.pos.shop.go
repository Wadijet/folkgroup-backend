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

// PcPosShopHandler xử lý các yêu cầu liên quan đến Pancake POS Shop
type PcPosShopHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput]
	PcPosShopService *pcsvc.PcPosShopService
}

// NewPcPosShopHandler khởi tạo PcPosShopHandler mới
func NewPcPosShopHandler() (*PcPosShopHandler, error) {
	service, err := pcsvc.NewPcPosShopService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos shop service: %v", err)
	}
	hdl := &PcPosShopHandler{PcPosShopService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// SyncUpsertOneFromParts filter + body — logic ở PcPosShopService.RunSyncUpsertOneFromJSON.
func (h *PcPosShopHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.PcPosShopService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
