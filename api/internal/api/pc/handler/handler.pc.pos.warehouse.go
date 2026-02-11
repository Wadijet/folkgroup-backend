package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// PcPosWarehouseHandler xử lý các yêu cầu liên quan đến Pancake POS Warehouse
type PcPosWarehouseHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput]
	PcPosWarehouseService *pcsvc.PcPosWarehouseService
}

// NewPcPosWarehouseHandler khởi tạo PcPosWarehouseHandler mới
func NewPcPosWarehouseHandler() (*PcPosWarehouseHandler, error) {
	service, err := pcsvc.NewPcPosWarehouseService()
	if err != nil {
		return nil, fmt.Errorf("failed to create pc pos warehouse service: %v", err)
	}
	hdl := &PcPosWarehouseHandler{PcPosWarehouseService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}
