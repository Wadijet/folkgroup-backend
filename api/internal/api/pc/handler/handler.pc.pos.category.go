package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcPosCategoryHandler xử lý các yêu cầu liên quan đến Pancake POS Category
type PcPosCategoryHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput]
	PcPosCategoryService *pcsvc.PcPosCategoryService
}

// NewPcPosCategoryHandler khởi tạo PcPosCategoryHandler mới
func NewPcPosCategoryHandler() (*PcPosCategoryHandler, error) {
	service, err := pcsvc.NewPcPosCategoryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos category service: %v", err)
	}
	hdl := &PcPosCategoryHandler{PcPosCategoryService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}
