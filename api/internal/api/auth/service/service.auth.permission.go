// Package authsvc - service quyền (Permission).
package authsvc

import (
	"fmt"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PermissionService là cấu trúc chứa các phương thức liên quan đến quyền
type PermissionService struct {
	*basesvc.BaseServiceMongoImpl[models.Permission]
}

// NewPermissionService tạo mới PermissionService
func NewPermissionService() (*PermissionService, error) {
	permissionCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Permissions)
	if !exist {
		return nil, fmt.Errorf("failed to get permissions collection: %v", common.ErrNotFound)
	}

	return &PermissionService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.Permission](permissionCollection),
	}, nil
}
