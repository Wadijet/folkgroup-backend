package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// LogicScript document lưu trong collection rule_logic_definitions.
// Logic Script là artifact reasoning có version. Toàn bộ logic rule triển khai trong script.
type LogicScript struct {
	LogicID       string            `json:"logic_id" bson:"logic_id" index:"single:1"`
	LogicVersion  int               `json:"logic_version" bson:"logic_version"`
	LogicType     string            `json:"logic_type" bson:"logic_type"` // "script"
	Runtime       string            `json:"runtime" bson:"runtime"`       // "goja"
	EntryFunction string            `json:"entry_function" bson:"entry_function"`
	SourceHash    string            `json:"source_hash,omitempty" bson:"source_hash,omitempty"`
	ChangeReason  string            `json:"change_reason,omitempty" bson:"change_reason,omitempty"`
	Status                 string                 `json:"status" bson:"status" index:"single:1"`
	OwnerOrganizationID    primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	IsSystem               bool                   `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa
	Script                 string                 `json:"script" bson:"script"`
	Metadata               map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt              int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt              int64                  `json:"updatedAt" bson:"updatedAt"`
}
