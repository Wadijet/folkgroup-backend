// Package handler — CRUD handler cho Rule Definition.
package handler

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/ruleintel/dto"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/api/ruleintel/service"
)

// RuleDefinitionHandler CRUD cho Rule Definition.
type RuleDefinitionHandler struct {
	*basehdl.BaseHandler[models.RuleDefinition, dto.RuleDefinitionCreateInput, dto.RuleDefinitionUpdateInput]
}

// NewRuleDefinitionHandler tạo RuleDefinitionHandler.
func NewRuleDefinitionHandler() (*RuleDefinitionHandler, error) {
	svc, err := service.NewRuleDefinitionService()
	if err != nil {
		return nil, fmt.Errorf("tạo RuleDefinitionService: %w", err)
	}
	h := &RuleDefinitionHandler{
		BaseHandler: basehdl.NewBaseHandler[models.RuleDefinition, dto.RuleDefinitionCreateInput, dto.RuleDefinitionUpdateInput](svc),
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"isSystem"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return h, nil
}
