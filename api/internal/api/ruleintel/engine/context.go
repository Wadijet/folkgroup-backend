// Package engine — Rule Engine core cho Rule Intelligence.
//
// Chạy Logic Script (goja), validate output, ghi trace.
package engine

import "meta_commerce/internal/api/ruleintel/models"

// EvalContext context truyền vào Logic Script.
// Script chỉ đọc ctx — không ghi DB, không gọi API.
type EvalContext struct {
	Layers    map[string]interface{} `json:"layers"`
	Params    map[string]interface{} `json:"params"`
	EntityRef models.EntityRef      `json:"entity_ref"`
}

// EvalResult kết quả script trả về.
type EvalResult struct {
	Output interface{}            `json:"output"`
	Report map[string]interface{} `json:"report"`
}

// RunResult kết quả Rule Engine trả về cho module gọi.
type RunResult struct {
	OutputType   string                 `json:"output_type"`
	Result       interface{}            `json:"result"`
	Report       map[string]interface{} `json:"report"`
	EntityRef    models.EntityRef       `json:"entity_ref"`
	RuleID       string                 `json:"rule_id"`
	RuleCode     string                 `json:"rule_code"`
	TraceID      string                 `json:"trace_id"`
	LogicID      string                 `json:"logic_id"`
	LogicVersion int                    `json:"logic_version"`
	ParamSetID   string                 `json:"param_set_id"`
	ParamVersion int                    `json:"param_version"`
}
