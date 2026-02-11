package authdto

// OrganizationConfigItemUpsertInput body cho upsert một config item.
type OrganizationConfigItemUpsertInput struct {
	OwnerOrganizationID string      `json:"ownerOrganizationId" validate:"required" transform:"str_objectid"`
	Key                 string      `json:"key"`
	Value               interface{} `json:"value"`
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	DataType            string      `json:"dataType"`
	Constraints         string      `json:"constraints,omitempty"`
	AllowOverride       bool        `json:"allowOverride"`
}
