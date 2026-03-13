// Package dto — DTO cho Rule Definition CRUD.
package dto

import "meta_commerce/internal/api/ruleintel/models"

// RuleDefinitionCreateInput input tạo Rule Definition.
type RuleDefinitionCreateInput struct {
	RuleID               string                 `json:"ruleId" validate:"required"`
	RuleVersion          int                    `json:"ruleVersion" validate:"required"`
	RuleCode             string                 `json:"ruleCode" validate:"required"`
	Domain               string                 `json:"domain" validate:"required"`
	FromLayer            string                 `json:"fromLayer" validate:"required"`
	ToLayer              string                 `json:"toLayer" validate:"required"`
	InputRef             models.InputRef        `json:"inputRef"`
	LogicRef             models.LogicRef        `json:"logicRef" validate:"required"`
	ParamRef             models.ParamRef        `json:"paramRef" validate:"required"`
	OutputRef            models.OutputRef       `json:"outputRef" validate:"required"`
	Priority             int                    `json:"priority"`
	Status               string                 `json:"status"`
	OwnerOrganizationID  string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	Metadata             map[string]string     `json:"metadata,omitempty"`
}

// RuleDefinitionUpdateInput input cập nhật Rule Definition.
type RuleDefinitionUpdateInput struct {
	RuleVersion          *int                   `json:"ruleVersion,omitempty"`
	RuleCode             *string                `json:"ruleCode,omitempty"`
	FromLayer            *string                `json:"fromLayer,omitempty"`
	ToLayer              *string                `json:"toLayer,omitempty"`
	InputRef             *models.InputRef       `json:"inputRef,omitempty"`
	LogicRef             *models.LogicRef       `json:"logicRef,omitempty"`
	ParamRef             *models.ParamRef       `json:"paramRef,omitempty"`
	OutputRef            *models.OutputRef      `json:"outputRef,omitempty"`
	Priority             *int                   `json:"priority,omitempty"`
	Status               *string                `json:"status,omitempty"`
	Metadata             *map[string]string     `json:"metadata,omitempty"`
}
