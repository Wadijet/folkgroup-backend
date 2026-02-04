package dto

// OrganizationConfigItemUpsertInput body cho upsert một config item (1 key).
// Dùng cho POST /organization-config/upsert-one.
// ownerOrganizationId dạng string trong JSON được convert sang primitive.ObjectID nhờ struct tag transform.
type OrganizationConfigItemUpsertInput struct {
	OwnerOrganizationID string      `json:"ownerOrganizationId" validate:"required" transform:"str_objectid"` // Tổ chức sở hữu config - tự động convert string → ObjectID
	Key                 string      `json:"key"`
	Value               interface{} `json:"value"`
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	DataType            string      `json:"dataType"`
	Constraints         string      `json:"constraints,omitempty"`
	AllowOverride       bool        `json:"allowOverride"`
}
