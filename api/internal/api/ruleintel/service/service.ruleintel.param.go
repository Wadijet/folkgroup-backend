// Package service — ParamSetService CRUD cho Parameter Set.
package service

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/api/ruleintel/models"
)

// ParamSetService CRUD cho Parameter Set.
type ParamSetService struct {
	*basesvc.BaseServiceMongoImpl[models.ParamSet]
}

// NewParamSetService tạo ParamSetService.
func NewParamSetService() (*ParamSetService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleParamSets)
	if !ok {
		return nil, fmt.Errorf("collection %s chưa đăng ký: %w", global.MongoDB_ColNames.RuleParamSets, common.ErrNotFound)
	}
	return &ParamSetService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.ParamSet](coll),
	}, nil
}
