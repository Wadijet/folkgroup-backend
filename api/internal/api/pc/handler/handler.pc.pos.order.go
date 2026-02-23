package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcPosOrderHandler xử lý các yêu cầu liên quan đến Pancake POS Order
type PcPosOrderHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput]
	PcPosOrderService *pcsvc.PcPosOrderService
}

// NewPcPosOrderHandler khởi tạo PcPosOrderHandler mới
func NewPcPosOrderHandler() (*PcPosOrderHandler, error) {
	service, err := pcsvc.NewPcPosOrderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos order service: %v", err)
	}
	hdl := &PcPosOrderHandler{PcPosOrderService: service}
	// Dùng full service để CRUD đi qua BaseServiceMongoImpl (đã tích hợp EmitDataChanged)
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput](service)
	return hdl, nil
}
