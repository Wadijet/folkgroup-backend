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

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosProduct struct để extract chạy (flatten posData → productId, shopId, name, ...).
func (h *PcPosProductHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var product pcmodels.PcPosProduct
	if err := json.Unmarshal(c.Body(), &product); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && product.OwnerOrganizationID.IsZero() {
		product.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&product); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosProductService.SyncUpsertOne(c.Context(), filter, &product)
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
