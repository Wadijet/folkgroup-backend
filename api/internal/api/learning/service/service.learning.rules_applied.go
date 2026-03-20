// Package learningsvc — Query rule_execution_logs theo trace_id để lấy rules_applied.
package learningsvc

import (
	"context"

	"meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
)

// ruleExecutionLogDoc cấu trúc document trong rule_execution_logs (flexible — field có thể khác tên).
type ruleExecutionLogDoc struct {
	TraceID       string `bson:"traceId,omitempty"`
	TraceIDAlt    string `bson:"trace_id,omitempty"`
	RuleID        string `bson:"ruleId,omitempty"`
	RuleIDAlt     string `bson:"rule_id,omitempty"`
	LogicVersion  int    `bson:"logicVersion,omitempty"`
	LogicVersionAlt int  `bson:"logic_version,omitempty"`
	ParamVersion  string `bson:"paramVersion,omitempty"`
	ParamVersionAlt string `bson:"param_version,omitempty"`
	Output        interface{} `bson:"output,omitempty"`
	OutputObject  interface{} `bson:"output_object,omitempty"`
}

// FetchRulesAppliedFromTraceID query rule_execution_logs theo traceId, trả về rules_applied.
func FetchRulesAppliedFromTraceID(ctx context.Context, traceID string) ([]models.RuleAppliedEntry, string) {
	if traceID == "" {
		return nil, ""
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleExecutionLogs)
	if !ok {
		return nil, ""
	}
	filter := bson.M{
		"$or": []bson.M{
			{"traceId": traceID},
			{"trace_id": traceID},
		},
	}
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, ""
	}
	defer cursor.Close(ctx)

	var entries []models.RuleAppliedEntry
	var paramVersion string
	for cursor.Next(ctx) {
		var doc ruleExecutionLogDoc
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		ruleID := doc.RuleID
		if ruleID == "" {
			ruleID = doc.RuleIDAlt
		}
		logicVer := doc.LogicVersion
		if logicVer == 0 {
			logicVer = doc.LogicVersionAlt
		}
		if doc.ParamVersion != "" {
			paramVersion = doc.ParamVersion
		} else if doc.ParamVersionAlt != "" {
			paramVersion = doc.ParamVersionAlt
		}
		outputStr := ""
		if doc.Output != nil {
			if s, ok := doc.Output.(string); ok {
				outputStr = s
			}
		}
		if outputStr == "" && doc.OutputObject != nil {
			if m, ok := doc.OutputObject.(map[string]interface{}); ok {
				if a, ok := m["action"].(string); ok {
					outputStr = a
				}
			}
		}
		entries = append(entries, models.RuleAppliedEntry{
			RuleID:       ruleID,
			LogicVersion: logicVer,
			Output:       outputStr,
		})
	}
	return entries, paramVersion
}
