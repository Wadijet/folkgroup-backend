// Package service — Service cho RuleExecutionTrace, dùng BaseServiceMongoImpl để áp dụng hooks.
package service

import (
	"fmt"

	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	basesvc "meta_commerce/internal/api/base/service"
)

// RuleExecutionTraceService quản lý rule_execution_logs qua BaseServiceMongoImpl.
type RuleExecutionTraceService struct {
	*basesvc.BaseServiceMongoImpl[ruleintelmodels.RuleExecutionTrace]
}

// NewRuleExecutionTraceService tạo RuleExecutionTraceService mới.
func NewRuleExecutionTraceService() (*RuleExecutionTraceService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleExecutionLogs)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.RuleExecutionLogs, common.ErrNotFound)
	}
	return &RuleExecutionTraceService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[ruleintelmodels.RuleExecutionTrace](coll),
	}, nil
}
