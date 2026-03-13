package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

const (
	// ScriptTimeout thời gian tối đa chạy script (ms).
	ScriptTimeout = 100
)

// ScriptExecutor chạy Logic Script với goja.
type ScriptExecutor struct {
	entryFunction string
}

// NewScriptExecutor tạo executor với entry function name.
func NewScriptExecutor(entryFunction string) *ScriptExecutor {
	if entryFunction == "" {
		entryFunction = "evaluate"
	}
	return &ScriptExecutor{entryFunction: entryFunction}
}

// Run chạy script với context, trả về output và report.
func (e *ScriptExecutor) Run(ctx context.Context, script string, evalCtx *EvalContext) (*EvalResult, int64, error) {
	vm := goja.New()
	start := time.Now()

	// Expose ctx vào VM
	ctxObj := vm.NewObject()
	layersObj := vm.NewObject()
	for k, v := range evalCtx.Layers {
		layersObj.Set(k, v)
	}
	ctxObj.Set("layers", layersObj)

	paramsObj := vm.NewObject()
	for k, v := range evalCtx.Params {
		paramsObj.Set(k, v)
	}
	ctxObj.Set("params", paramsObj)

	entityRefObj := vm.NewObject()
	entityRefObj.Set("domain", evalCtx.EntityRef.Domain)
	entityRefObj.Set("objectType", evalCtx.EntityRef.ObjectType)
	entityRefObj.Set("objectId", evalCtx.EntityRef.ObjectID)
	entityRefObj.Set("ownerOrganizationId", evalCtx.EntityRef.OwnerOrganizationID)
	ctxObj.Set("entity_ref", entityRefObj)

	vm.Set("ctx", ctxObj)

	// Chạy script
	_, err := vm.RunString(script)
	if err != nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("script parse/load: %w", err)
	}

	// Gọi entry function
	entryFn, ok := goja.AssertFunction(vm.Get(e.entryFunction))
	if !ok {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("entry function %s không tồn tại", e.entryFunction)
	}

	result, err := entryFn(goja.Undefined(), ctxObj)
	if err != nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("script execution: %w", err)
	}

	// Parse result { output, report }
	if result == nil || goja.IsNull(result) || goja.IsUndefined(result) {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("script không trả về kết quả")
	}

	resultObj, ok := result.Export().(map[string]interface{})
	if !ok {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("script trả về không phải object")
	}

	output, _ := resultObj["output"]
	report, _ := resultObj["report"]
	if report == nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("script bắt buộc trả về report")
	}

	reportMap, ok := report.(map[string]interface{})
	if !ok {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("report phải là object")
	}

	// Kiểm tra report có log
	if _, hasLog := reportMap["log"]; !hasLog {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("report phải có field log")
	}

	return &EvalResult{
		Output: output,
		Report: reportMap,
	}, time.Since(start).Milliseconds(), nil
}
