package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
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
