package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcOrderHandler xử lý các yêu cầu liên quan đến đơn hàng
type PcOrderHandler struct {
	*basehdl.BaseHandler[pcmodels.PcOrder, pcdto.PcOrderCreateInput, pcdto.PcOrderCreateInput]
	PcOrderService *pcsvc.PcOrderService
}

// NewPcOrderHandler khởi tạo PcOrderHandler mới
func NewPcOrderHandler() (*PcOrderHandler, error) {
	service, err := pcsvc.NewPcOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create order service: %v", err)
	}
	hdl := &PcOrderHandler{
		BaseHandler:    basehdl.NewBaseHandler[pcmodels.PcOrder, pcdto.PcOrderCreateInput, pcdto.PcOrderCreateInput](service.BaseServiceMongoImpl),
		PcOrderService: service,
	}
	return hdl, nil
}
