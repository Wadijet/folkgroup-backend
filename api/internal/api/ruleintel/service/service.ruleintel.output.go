// Package service — OutputContractService CRUD cho Output Contract.
package service

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/api/ruleintel/models"
)

// OutputContractService CRUD cho Output Contract.
type OutputContractService struct {
	*basesvc.BaseServiceMongoImpl[models.OutputContract]
}

// NewOutputContractService tạo OutputContractService.
func NewOutputContractService() (*OutputContractService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleOutputDefinitions)
	if !ok {
		return nil, fmt.Errorf("collection %s chưa đăng ký: %w", global.MongoDB_ColNames.RuleOutputDefinitions, common.ErrNotFound)
	}
	return &OutputContractService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.OutputContract](coll),
	}, nil
}
