package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcPosShopHandler xử lý các yêu cầu liên quan đến Pancake POS Shop
type PcPosShopHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput]
	PcPosShopService *pcsvc.PcPosShopService
}

// NewPcPosShopHandler khởi tạo PcPosShopHandler mới
func NewPcPosShopHandler() (*PcPosShopHandler, error) {
	service, err := pcsvc.NewPcPosShopService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos shop service: %v", err)
	}
	hdl := &PcPosShopHandler{PcPosShopService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}
