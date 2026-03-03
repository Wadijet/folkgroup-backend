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

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosCategory struct để extract chạy (flatten posData → categoryId, shopId, name, ...).
func (h *PcPosCategoryHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var category pcmodels.PcPosCategory
	if err := json.Unmarshal(c.Body(), &category); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && category.OwnerOrganizationID.IsZero() {
		category.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&category); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosCategoryService.SyncUpsertOne(c.Context(), filter, &category)
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
