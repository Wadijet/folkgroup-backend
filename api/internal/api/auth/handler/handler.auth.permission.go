package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	models "meta_commerce/internal/api/auth/models"
)

// PermissionHandler xử lý các route liên quan đến permission
type PermissionHandler struct {
	*basehdl.BaseHandler[models.Permission, authdto.PermissionCreateInput, authdto.PermissionUpdateInput]
}

// NewPermissionHandler tạo instance mới của PermissionHandler
func NewPermissionHandler() (*PermissionHandler, error) {
	permissionService, err := authsvc.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	return &PermissionHandler{
		BaseHandler: basehdl.NewBaseHandler[models.Permission, authdto.PermissionCreateInput, authdto.PermissionUpdateInput](permissionService),
	}, nil
}
