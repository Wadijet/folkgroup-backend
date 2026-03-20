// Package service — Service cho Rule Intelligence.
//
// Rule Engine: load rule, resolve logic/param/output, chạy Logic Script, ghi trace.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"

	"meta_commerce/internal/api/ruleintel/engine"
	"meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/common"
)

// RuleEngineService service chạy Rule Engine.
type RuleEngineService struct {
	executor       *engine.ScriptExecutor
	ruleSvc        *RuleDefinitionService
	logicSvc       *LogicScriptService
	paramSvc       *ParamSetService
	outputSvc      *OutputContractService
	traceSvc       *RuleExecutionTraceService
}

// NewRuleEngineService tạo service.
func NewRuleEngineService() (*RuleEngineService, error) {
	ruleSvc, err := NewRuleDefinitionService()
	if err != nil {
		return nil, fmt.Errorf("RuleDefinitionService: %w", err)
	}
	logicSvc, err := NewLogicScriptService()
	if err != nil {
		return nil, fmt.Errorf("LogicScriptService: %w", err)
	}
	paramSvc, err := NewParamSetService()
	if err != nil {
		return nil, fmt.Errorf("ParamSetService: %w", err)
	}
	outputSvc, err := NewOutputContractService()
	if err != nil {
		return nil, fmt.Errorf("OutputContractService: %w", err)
	}
	traceSvc, err := NewRuleExecutionTraceService()
	if err != nil {
		return nil, fmt.Errorf("RuleExecutionTraceService: %w", err)
	}
	return &RuleEngineService{
		executor:  engine.NewScriptExecutor("evaluate"),
		ruleSvc:   ruleSvc,
		logicSvc:  logicSvc,
		paramSvc:  paramSvc,
		outputSvc: outputSvc,
		traceSvc:  traceSvc,
	}, nil
}

// RunInput input khi gọi Rule Engine.
type RunInput struct {
	RuleID        string                 `json:"rule_id"`
	Domain        string                 `json:"domain"`
	EntityRef     models.EntityRef      `json:"entity_ref"`
	Layers        map[string]interface{} `json:"layers"`
	ParamsOverride map[string]interface{} `json:"params_override,omitempty"`
}

// Run chạy rule theo rule_id, trả về output và report.
func (s *RuleEngineService) Run(ctx context.Context, input *RunInput) (*engine.RunResult, error) {
	// 1. Load Rule Definition
	rule, err := s.loadRule(ctx, input.RuleID, input.Domain)
	if err != nil {
		return nil, err
	}

	// 2. Load Logic Script
	logic, err := s.loadLogic(ctx, rule.LogicRef.LogicID, rule.LogicRef.LogicVersion)
	if err != nil {
		return nil, err
	}

	// 3. Load Parameter Set
	params, err := s.loadParams(ctx, rule.ParamRef.ParamSetID, rule.ParamRef.ParamVersion)
	if err != nil {
		return nil, err
	}

	// Merge params_override
	for k, v := range input.ParamsOverride {
		params[k] = v
	}

	// 4. Load Output Contract (để validate, có thể bỏ qua nếu chưa implement validation)
	_, _ = s.loadOutput(ctx, rule.OutputRef.OutputID, rule.OutputRef.OutputVersion)

	// 5. Build EvalContext
	evalCtx := &engine.EvalContext{
		Layers:    input.Layers,
		Params:    params,
		EntityRef: input.EntityRef,
	}

	// 6. Run script
	traceID := uuid.New().String()
	evalResult, execTime, err := s.executor.Run(ctx, logic.Script, evalCtx)

	now := time.Now().UnixMilli()
	status := "success"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}

	// 7. Ghi trace — mọi lần chạy đều phải có explanation.log (audit, debug)
	trace := &models.RuleExecutionTrace{
		TraceID:            traceID,
		RuleID:             rule.RuleID,
		RuleVersion:        rule.RuleVersion,
		LogicID:            logic.LogicID,
		LogicVersion:       logic.LogicVersion,
		ParamSetID:         rule.ParamRef.ParamSetID,
		ParamVersion:       rule.ParamRef.ParamVersion,
		InputSnapshot:     input.Layers,
		ParametersSnapshot: params,
		OutputObject:       nil,
		ExecutionStatus:    status,
		ErrorMessage:      errMsg,
		Explanation:       nil,
		ExecutionTime:     execTime,
		Timestamp:         now,
		EntityRef:         input.EntityRef,
	}

	if evalResult != nil {
		trace.OutputObject = evalResult.Output
		trace.Explanation = evalResult.Report
	} else if err != nil {
		// Khi lỗi: vẫn ghi explanation để có log cho mọi lần chạy
		trace.Explanation = map[string]interface{}{"log": errMsg, "result": "error"}
	}

	if err := s.saveTrace(ctx, trace); err != nil {
		// Log nhưng không fail
		_ = err
	}

	if err != nil {
		return nil, err
	}

	// 8. Build RunResult
	outputType := "action"
	if oc, _ := s.loadOutput(ctx, rule.OutputRef.OutputID, rule.OutputRef.OutputVersion); oc != nil {
		outputType = oc.OutputType
	}

	return &engine.RunResult{
		OutputType:   outputType,
		Result:       evalResult.Output,
		Report:       evalResult.Report,
		EntityRef:    input.EntityRef,
		RuleID:       rule.RuleID,
		RuleCode:     rule.RuleCode,
		TraceID:      traceID,
		LogicID:      logic.LogicID,
		LogicVersion: logic.LogicVersion,
		ParamSetID:   rule.ParamRef.ParamSetID,
		ParamVersion: rule.ParamRef.ParamVersion,
	}, nil
}

func (s *RuleEngineService) loadRule(ctx context.Context, ruleID, domain string) (*models.RuleDefinition, error) {
	filter := bson.M{"rule_id": ruleID, "domain": domain, "status": "active"}
	rule, err := s.ruleSvc.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, fmt.Errorf("không tìm thấy rule %s: %w", ruleID, common.ErrNotFound)
		}
		return nil, err
	}
	return &rule, nil
}

func (s *RuleEngineService) loadLogic(ctx context.Context, logicID string, logicVersion int) (*models.LogicScript, error) {
	filter := bson.M{"logic_id": logicID, "logic_version": logicVersion, "status": "active"}
	logic, err := s.logicSvc.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, fmt.Errorf("không tìm thấy logic %s v%d: %w", logicID, logicVersion, common.ErrNotFound)
		}
		return nil, err
	}
	return &logic, nil
}

func (s *RuleEngineService) loadParams(ctx context.Context, paramSetID string, paramVersion int) (map[string]interface{}, error) {
	filter := bson.M{"param_set_id": paramSetID, "param_version": paramVersion}
	ps, err := s.paramSvc.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, fmt.Errorf("không tìm thấy param set %s v%d: %w", paramSetID, paramVersion, common.ErrNotFound)
		}
		return nil, err
	}
	if ps.Parameters == nil {
		return map[string]interface{}{}, nil
	}
	return ps.Parameters, nil
}

func (s *RuleEngineService) loadOutput(ctx context.Context, outputID string, outputVersion int) (*models.OutputContract, error) {
	filter := bson.M{"output_id": outputID, "output_version": outputVersion}
	oc, err := s.outputSvc.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, nil
	}
	return &oc, nil
}

func (s *RuleEngineService) saveTrace(ctx context.Context, trace *models.RuleExecutionTrace) error {
	_, err := s.traceSvc.InsertOne(ctx, *trace)
	return err
}

// FindTraceByTraceID tìm rule execution log theo trace_id. Dùng cho link "Xem log tạo đề xuất" từ proposal.
func (s *RuleEngineService) FindTraceByTraceID(ctx context.Context, traceID string) (*models.RuleExecutionTrace, error) {
	if traceID == "" {
		return nil, common.ErrNotFound
	}
	trace, err := s.traceSvc.FindOne(ctx, bson.M{"trace_id": traceID}, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &trace, nil
}
