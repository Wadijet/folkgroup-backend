package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// OutputContract document lưu trong collection rule_output_definitions.
// Output Contract định nghĩa schema output mà Logic Script phải tuân thủ.
type OutputContract struct {
	OutputID         string                 `json:"output_id" bson:"output_id" index:"single:1"`
	OutputVersion    int                    `json:"output_version" bson:"output_version"`
	OutputType       string                 `json:"output_type" bson:"output_type"`
	SchemaDefinition map[string]interface{} `json:"schema_definition" bson:"schema_definition"`
	RequiredFields      []string               `json:"required_fields,omitempty" bson:"required_fields,omitempty"`
	ValidationRules     []string               `json:"validation_rules,omitempty" bson:"validation_rules,omitempty"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	IsSystem            bool                   `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
}
