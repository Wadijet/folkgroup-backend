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

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosVariation struct để extract chạy (flatten posData → variationId, productId, sku, ...).
func (h *PcPosVariationHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var variation pcmodels.PcPosVariation
	if err := json.Unmarshal(c.Body(), &variation); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && variation.OwnerOrganizationID.IsZero() {
		variation.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&variation); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosVariationService.SyncUpsertOne(c.Context(), filter, &variation)
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
