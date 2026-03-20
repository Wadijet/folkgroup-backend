package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EntityRef tham chiếu entity trong context.
type EntityRef struct {
	Domain               string `json:"domain" bson:"domain"`
	ObjectType           string `json:"objectType" bson:"objectType"`
	ObjectID             string `json:"objectId" bson:"objectId"`
	OwnerOrganizationID  string `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
}

// RuleExecutionTrace document lưu trong collection rule_execution_logs.
// Full rule execution trace cho mỗi lần chạy — phục vụ debugging, audit, observability.
type RuleExecutionTrace struct {
	ID                 primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	TraceID            string                 `json:"trace_id" bson:"trace_id" index:"unique:1"`
	RuleID             string                 `json:"rule_id" bson:"rule_id" index:"single:1"`
	RuleVersion        int                    `json:"rule_version" bson:"rule_version"`
	LogicID            string                 `json:"logic_id" bson:"logic_id"`
	LogicVersion       int                    `json:"logic_version" bson:"logic_version"`
	ParamSetID         string                 `json:"param_set_id" bson:"param_set_id"`
	ParamVersion       int                    `json:"param_version" bson:"param_version"`
	InputSnapshot      map[string]interface{} `json:"input_snapshot" bson:"input_snapshot"`
	ParametersSnapshot map[string]interface{} `json:"parameters_snapshot" bson:"parameters_snapshot"`
	OutputObject       interface{}            `json:"output_object,omitempty" bson:"output_object,omitempty"`
	ExecutionStatus    string                 `json:"execution_status" bson:"execution_status" index:"single:1"`
	ErrorMessage       string                 `json:"error_message,omitempty" bson:"error_message,omitempty"`
	Explanation        map[string]interface{} `json:"explanation" bson:"explanation"`
	ExecutionTime      int64                  `json:"execution_time" bson:"execution_time"`
	Timestamp          int64                  `json:"timestamp" bson:"timestamp" index:"single:1"`
	EntityRef          EntityRef              `json:"entity_ref" bson:"entity_ref"`
}
