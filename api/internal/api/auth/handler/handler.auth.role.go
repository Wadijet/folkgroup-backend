package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	models "meta_commerce/internal/api/auth/models"
)

// RoleHandler xử lý các route liên quan đến vai trò
type RoleHandler struct {
	*basehdl.BaseHandler[models.Role, authdto.RoleCreateInput, authdto.RoleUpdateInput]
	RoleService *authsvc.RoleService
}

// NewRoleHandler tạo instance mới của RoleHandler
func NewRoleHandler() (*RoleHandler, error) {
	roleService, err := authsvc.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	return &RoleHandler{
		BaseHandler: basehdl.NewBaseHandler[models.Role, authdto.RoleCreateInput, authdto.RoleUpdateInput](roleService),
		RoleService: roleService,
	}, nil
}
