package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcPosVariationHandler xử lý các yêu cầu liên quan đến Pancake POS Variation
type PcPosVariationHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput]
	PcPosVariationService *pcsvc.PcPosVariationService
}

// NewPcPosVariationHandler khởi tạo PcPosVariationHandler mới
func NewPcPosVariationHandler() (*PcPosVariationHandler, error) {
	service, err := pcsvc.NewPcPosVariationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos variation service: %v", err)
	}
	hdl := &PcPosVariationHandler{PcPosVariationService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}
