// Package service — LogicScriptService CRUD cho Logic Script.
package service

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/api/ruleintel/models"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// LogicScriptService CRUD cho Logic Script.
type LogicScriptService struct {
	*basesvc.BaseServiceMongoImpl[models.LogicScript]
}

// NewLogicScriptService tạo LogicScriptService.
func NewLogicScriptService() (*LogicScriptService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleLogicDefinitions)
	if !ok {
		return nil, fmt.Errorf("collection %s chưa đăng ký: %w", global.MongoDB_ColNames.RuleLogicDefinitions, common.ErrNotFound)
	}
	return &LogicScriptService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.LogicScript](coll),
	}, nil
}

// FindOne tìm Logic Script theo filter.
func (s *LogicScriptService) FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (models.LogicScript, error) {
	return s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
}
