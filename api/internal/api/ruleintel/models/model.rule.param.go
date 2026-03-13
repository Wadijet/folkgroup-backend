package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// ParamSet document lưu trong collection rule_param_sets.
// Parameter Set định nghĩa giá trị cấu hình có thể tune — version độc lập với Logic Script.
type ParamSet struct {
	ParamSetID   string            `json:"param_set_id" bson:"param_set_id" index:"single:1"`
	ParamVersion int               `json:"param_version" bson:"param_version"`
	Parameters   map[string]interface{} `json:"parameters" bson:"parameters"`
	Domain                string                 `json:"domain" bson:"domain" index:"single:1"`
	Segment               string                 `json:"segment" bson:"segment"`
	OwnerOrganizationID   primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	IsSystem              bool                   `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa
	Metadata              map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt             int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt             int64                  `json:"updatedAt" bson:"updatedAt"`
}
