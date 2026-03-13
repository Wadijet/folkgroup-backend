// Package handler — CRUD handler cho Param Set.
package handler

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/ruleintel/dto"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// ParamSetHandler CRUD cho Param Set.
type ParamSetHandler struct {
	*basehdl.BaseHandler[models.ParamSet, dto.ParamSetCreateInput, dto.ParamSetUpdateInput]
}

// NewParamSetHandler tạo ParamSetHandler.
func NewParamSetHandler() (*ParamSetHandler, error) {
	svc, err := service.NewParamSetService()
	if err != nil {
		return nil, fmt.Errorf("tạo ParamSetService: %w", err)
	}
	h := &ParamSetHandler{
		BaseHandler: basehdl.NewBaseHandler[models.ParamSet, dto.ParamSetCreateInput, dto.ParamSetUpdateInput](svc),
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"isSystem"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return h, nil
}
