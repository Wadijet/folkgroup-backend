package fbhdl

import (
	"fmt"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	basehdl "meta_commerce/internal/api/base/handler"
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
