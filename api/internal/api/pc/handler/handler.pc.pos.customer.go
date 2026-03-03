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

// PcPosCustomerHandler xử lý các route liên quan đến Pancake POS Customer
type PcPosCustomerHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosCustomer, pcdto.PcPosCustomerCreateInput, pcdto.PcPosCustomerCreateInput]
	PcPosCustomerService *pcsvc.PcPosCustomerService
}

// NewPcPosCustomerHandler tạo PcPosCustomerHandler mới
func NewPcPosCustomerHandler() (*PcPosCustomerHandler, error) {
	service, err := pcsvc.NewPcPosCustomerService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos customer service: %v", err)
	}
	hdl := &PcPosCustomerHandler{PcPosCustomerService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosCustomer, pcdto.PcPosCustomerCreateInput, pcdto.PcPosCustomerCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào PcPosCustomer struct để extract chạy (flatten posData → customerId, shopId, name, ...).
func (h *PcPosCustomerHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var customer pcmodels.PcPosCustomer
	if err := json.Unmarshal(c.Body(), &customer); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && customer.OwnerOrganizationID.IsZero() {
		customer.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&customer); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.PcPosCustomerService.SyncUpsertOne(c.Context(), filter, &customer)
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
