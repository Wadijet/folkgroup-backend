// Package dto — DTO cho Param Set CRUD.
package dto

// ParamSetCreateInput input tạo Param Set.
type ParamSetCreateInput struct {
	ParamSetID           string                 `json:"paramSetId" validate:"required"`
	ParamVersion         int                    `json:"paramVersion" validate:"required"`
	Parameters           map[string]interface{} `json:"parameters"`
	Domain               string                 `json:"domain" validate:"required"`
	Segment              string                 `json:"segment"`
	OwnerOrganizationID  string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// ParamSetUpdateInput input cập nhật Param Set.
type ParamSetUpdateInput struct {
	ParamVersion         *int                   `json:"paramVersion,omitempty"`
	Parameters           *map[string]interface{} `json:"parameters,omitempty"`
	Domain               *string                `json:"domain,omitempty"`
	Segment              *string                `json:"segment,omitempty"`
	Metadata             *map[string]interface{} `json:"metadata,omitempty"`
}
