// Package dto — DTO cho Output Contract CRUD.
package dto

// OutputContractCreateInput input tạo Output Contract.
type OutputContractCreateInput struct {
	OutputID             string                 `json:"outputId" validate:"required"`
	OutputVersion        int                    `json:"outputVersion" validate:"required"`
	OutputType           string                 `json:"outputType" validate:"required"`
	SchemaDefinition     map[string]interface{} `json:"schemaDefinition"`
	RequiredFields       []string               `json:"requiredFields,omitempty"`
	ValidationRules      []string               `json:"validationRules,omitempty"`
	OwnerOrganizationID  string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// OutputContractUpdateInput input cập nhật Output Contract.
type OutputContractUpdateInput struct {
	OutputVersion        *int                   `json:"outputVersion,omitempty"`
	OutputType           *string                `json:"outputType,omitempty"`
	SchemaDefinition     *map[string]interface{} `json:"schemaDefinition,omitempty"`
	RequiredFields       *[]string              `json:"requiredFields,omitempty"`
	ValidationRules      *[]string              `json:"validationRules,omitempty"`
}
