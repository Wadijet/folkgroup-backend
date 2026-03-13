// Package handler — CRUD handler cho Output Contract.
package handler

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/ruleintel/dto"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// OutputContractHandler CRUD cho Output Contract.
type OutputContractHandler struct {
	*basehdl.BaseHandler[models.OutputContract, dto.OutputContractCreateInput, dto.OutputContractUpdateInput]
}

// NewOutputContractHandler tạo OutputContractHandler.
func NewOutputContractHandler() (*OutputContractHandler, error) {
	svc, err := service.NewOutputContractService()
	if err != nil {
		return nil, fmt.Errorf("tạo OutputContractService: %w", err)
	}
	h := &OutputContractHandler{
		BaseHandler: basehdl.NewBaseHandler[models.OutputContract, dto.OutputContractCreateInput, dto.OutputContractUpdateInput](svc),
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"isSystem"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return h, nil
}
