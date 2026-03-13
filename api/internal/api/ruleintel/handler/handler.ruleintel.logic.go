// Package handler — CRUD handler cho Logic Script.
package handler

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/ruleintel/dto"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// LogicScriptHandler CRUD cho Logic Script.
type LogicScriptHandler struct {
	*basehdl.BaseHandler[models.LogicScript, dto.LogicScriptCreateInput, dto.LogicScriptUpdateInput]
}

// NewLogicScriptHandler tạo LogicScriptHandler.
func NewLogicScriptHandler() (*LogicScriptHandler, error) {
	svc, err := service.NewLogicScriptService()
	if err != nil {
		return nil, fmt.Errorf("tạo LogicScriptService: %w", err)
	}
	h := &LogicScriptHandler{
		BaseHandler: basehdl.NewBaseHandler[models.LogicScript, dto.LogicScriptCreateInput, dto.LogicScriptUpdateInput](svc),
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"isSystem"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return h, nil
}
