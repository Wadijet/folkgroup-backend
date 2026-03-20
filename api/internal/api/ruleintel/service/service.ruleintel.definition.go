// Package service — RuleDefinitionService CRUD cho Rule Definition.
package service

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/api/ruleintel/models"
)

// RuleDefinitionService CRUD cho Rule Definition.
type RuleDefinitionService struct {
	*basesvc.BaseServiceMongoImpl[models.RuleDefinition]
}

// NewRuleDefinitionService tạo RuleDefinitionService.
func NewRuleDefinitionService() (*RuleDefinitionService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleDefinitions)
	if !ok {
		return nil, fmt.Errorf("collection %s chưa đăng ký: %w", global.MongoDB_ColNames.RuleDefinitions, common.ErrNotFound)
	}
	return &RuleDefinitionService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.RuleDefinition](coll),
	}, nil
}
