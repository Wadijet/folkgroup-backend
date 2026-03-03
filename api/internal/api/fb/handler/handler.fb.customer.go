package fbhdl

import (
	"encoding/json"
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
)

// FbCustomerHandler xử lý các route liên quan đến Facebook Customer
type FbCustomerHandler struct {
	*basehdl.BaseHandler[fbmodels.FbCustomer, fbdto.FbCustomerCreateInput, fbdto.FbCustomerCreateInput]
	FbCustomerService *fbsvc.FbCustomerService
}

// NewFbCustomerHandler tạo FbCustomerHandler mới
func NewFbCustomerHandler() (*FbCustomerHandler, error) {
	service, err := fbsvc.NewFbCustomerService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb customer service: %v", err)
	}
	hdl := &FbCustomerHandler{FbCustomerService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[fbmodels.FbCustomer, fbdto.FbCustomerCreateInput, fbdto.FbCustomerCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// HandleSyncUpsertOne xử lý sync-upsert-one: chỉ ghi khi dữ liệu mới hơn (giảm tải backend).
// Unmarshal vào FbCustomer struct để extract chạy (flatten panCakeData → customerId, psid, name, ...).
func (h *FbCustomerHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	filter, err := h.ProcessFilter(c)
	if err != nil {
		return err
	}
	var customer fbmodels.FbCustomer
	if err := json.Unmarshal(c.Body(), &customer); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if orgID := h.GetActiveOrganizationID(c); orgID != nil && !orgID.IsZero() && customer.OwnerOrganizationID.IsZero() {
		customer.OwnerOrganizationID = *orgID
	}
	if err := utility.ExtractDataIfExists(&customer); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	result, skipped, err := h.FbCustomerService.SyncUpsertOne(c.Context(), filter, &customer)
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
