package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
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
