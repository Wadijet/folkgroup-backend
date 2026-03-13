// Package dto — DTO cho Logic Script CRUD.
package dto

// LogicScriptCreateInput input tạo Logic Script.
type LogicScriptCreateInput struct {
	LogicID              string                 `json:"logicId" validate:"required"`
	LogicVersion         int                    `json:"logicVersion" validate:"required"`
	LogicType            string                 `json:"logicType"`
	Runtime              string                 `json:"runtime"`
	EntryFunction       string                 `json:"entryFunction"`
	Status               string                 `json:"status"`
	OwnerOrganizationID  string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	Script               string                 `json:"script" validate:"required"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// LogicScriptUpdateInput input cập nhật Logic Script.
type LogicScriptUpdateInput struct {
	LogicVersion         *int                   `json:"logicVersion,omitempty"`
	LogicType            *string                `json:"logicType,omitempty"`
	Runtime              *string                `json:"runtime,omitempty"`
	EntryFunction       *string                `json:"entryFunction,omitempty"`
	Status               *string                `json:"status,omitempty"`
	Script               *string                `json:"script,omitempty"`
	Metadata             *map[string]interface{} `json:"metadata,omitempty"`
}
