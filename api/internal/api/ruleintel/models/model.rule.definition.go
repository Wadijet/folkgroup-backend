// Package models — Model cho module Rule Intelligence.
//
// Rule Definition định nghĩa khi nào và ở đâu logic chạy — không chứa business logic.
// Rule tham chiếu Logic Script, Parameter Set, Output Contract.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// InputRef tham chiếu input schema.
type InputRef struct {
	SchemaRef      string   `json:"schema_ref" bson:"schema_ref"`
	RequiredFields []string `json:"required_fields,omitempty" bson:"required_fields,omitempty"`
}

// LogicRef tham chiếu Logic Script.
type LogicRef struct {
	LogicID     string `json:"logic_id" bson:"logic_id"`
	LogicVersion int    `json:"logic_version" bson:"logic_version"`
}

// ParamRef tham chiếu Parameter Set.
type ParamRef struct {
	ParamSetID string `json:"param_set_id" bson:"param_set_id"`
	ParamVersion int   `json:"param_version" bson:"param_version"`
}

// OutputRef tham chiếu Output Contract.
type OutputRef struct {
	OutputID     string `json:"output_id" bson:"output_id"`
	OutputVersion int   `json:"output_version" bson:"output_version"`
}

// RuleDefinition document lưu trong collection rule_definitions.
type RuleDefinition struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	RuleID     string             `json:"rule_id" bson:"rule_id" index:"unique:1"`
	RuleVersion int               `json:"rule_version" bson:"rule_version"`
	RuleCode   string             `json:"rule_code" bson:"rule_code" index:"single:1"`
	Domain     string             `json:"domain" bson:"domain" index:"single:1"`
	FromLayer  string             `json:"from_layer" bson:"from_layer"`
	ToLayer    string             `json:"to_layer" bson:"to_layer"`
	InputRef   InputRef           `json:"input_ref" bson:"input_ref"`
	LogicRef   LogicRef           `json:"logic_ref" bson:"logic_ref"`
	ParamRef   ParamRef           `json:"param_ref" bson:"param_ref"`
	OutputRef  OutputRef          `json:"output_ref" bson:"output_ref"`
	Priority               int                `json:"priority" bson:"priority"`
	Status                 string             `json:"status" bson:"status" index:"single:1"`
	OwnerOrganizationID    primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // System Org = rule hệ thống (seed)
	IsSystem               bool               `json:"-" bson:"isSystem" index:"single:1"`                               // true = dữ liệu hệ thống, không thể xóa
	Metadata               map[string]string  `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt              int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt              int64              `json:"updatedAt" bson:"updatedAt"`
}
