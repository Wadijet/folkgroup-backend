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

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosShop struct để extract chạy (flatten panCakeData → shopId, name, ...).
func (h *PcPosShopHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var shop pcmodels.PcPosShop
	if err := json.Unmarshal(c.Body(), &shop); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && shop.OwnerOrganizationID.IsZero() {
		shop.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&shop); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosShopService.SyncUpsertOne(c.Context(), filter, &shop)
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
