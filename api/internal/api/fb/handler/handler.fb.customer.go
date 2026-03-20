package fbhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	"meta_commerce/internal/common"

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

// SyncUpsertOneFromParts filter + body — logic ở FbCustomerService.RunSyncUpsertOneFromJSON.
func (h *FbCustomerHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.FbCustomerService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
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
