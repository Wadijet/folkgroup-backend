// Package authhdl - handler admin (block user, set role, sync permissions).
package authhdl

import (
	"fmt"

	authdto "meta_commerce/internal/api/auth/dto"
	authmodels "meta_commerce/internal/api/auth/models"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/initsvc"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminHandler xử lý các route liên quan đến quản trị viên
type AdminHandler struct {
	basehdl.BaseHandler[authmodels.User, authdto.UserCreateInput, authdto.UserChangeInfoInput]
	UserCRUD       *authsvc.UserService
	PermissionCRUD *authsvc.PermissionService
	RoleCRUD       *authsvc.RoleService
	AdminService   *authsvc.AdminService
}

// NewAdminHandler tạo một instance mới của AdminHandler
func NewAdminHandler() (*AdminHandler, error) {
	h := &AdminHandler{}
	userService, err := authsvc.NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}
	h.UserCRUD = userService
	permissionService, err := authsvc.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	h.PermissionCRUD = permissionService
	roleService, err := authsvc.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	h.RoleCRUD = roleService
	adminService, err := authsvc.NewAdminService()
	if err != nil {
		return nil, fmt.Errorf("failed to create admin service: %v", err)
	}
	h.AdminService = adminService
	h.BaseService = userService
	return h, nil
}

// SetRoleInput đầu vào thiết lập vai trò người dùng
type SetRoleInput struct {
	Email  string             `json:"email" validate:"required"`
	RoleID primitive.ObjectID `json:"roleID" validate:"required"`
}

// HandleSetRole xử lý thiết lập vai trò cho người dùng
func (h *AdminHandler) HandleSetRole(c fiber.Ctx) error {
	var input SetRoleInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, err.Error(), common.StatusBadRequest, nil))
		return nil
	}
	result, err := h.AdminService.SetRole(c.Context(), input.Email, input.RoleID)
	h.HandleResponse(c, result, err)
	return nil
}

// HandleBlockUser xử lý khóa người dùng
func (h *AdminHandler) HandleBlockUser(c fiber.Ctx) error {
	var input authdto.BlockUserInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, err.Error(), common.StatusBadRequest, nil))
		return nil
	}
	result, err := h.AdminService.BlockUser(c.Context(), input.Email, true, input.Note)
	h.HandleResponse(c, result, err)
	return nil
}

// HandleUnBlockUser xử lý mở khóa người dùng
func (h *AdminHandler) HandleUnBlockUser(c fiber.Ctx) error {
	var input authdto.UnBlockUserInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, err.Error(), common.StatusBadRequest, nil))
		return nil
	}
	result, err := h.AdminService.BlockUser(c.Context(), input.Email, false, "")
	h.HandleResponse(c, result, err)
	return nil
}

// HandleAddAdministrator thiết lập administrator (khi đã có admin). Yêu cầu quyền Init.SetAdmin
func (h *AdminHandler) HandleAddAdministrator(c fiber.Ctx) error {
	id := h.GetIDFromContext(c)
	if id == "" {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID không hợp lệ", common.StatusBadRequest, nil))
		return nil
	}
	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID không hợp lệ", common.StatusBadRequest, err))
		return nil
	}
	initService, err := initsvc.NewInitService()
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeInternalServer, "Không thể khởi tạo InitService", common.StatusInternalServerError, err))
		return nil
	}
	result, err := initService.SetAdministrator(userID)
	h.HandleResponse(c, result, err)
	return nil
}

// HandleSyncAdministratorPermissions đồng bộ quyền cho Administrator
func (h *AdminHandler) HandleSyncAdministratorPermissions(c fiber.Ctx) error {
	initService, err := initsvc.NewInitService()
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeInternalServer, "Không thể khởi tạo InitService", common.StatusInternalServerError, err))
		return nil
	}
	if err := initService.InitPermission(); err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeInternalServer, "Không thể khởi tạo permissions", common.StatusInternalServerError, err))
		return nil
	}
	if err := initService.CheckPermissionForAdministrator(); err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeInternalServer, "Không thể đồng bộ quyền cho Administrator", common.StatusInternalServerError, err))
		return nil
	}
	h.HandleResponse(c, map[string]string{"message": "Đã đồng bộ quyền cho Administrator thành công"}, nil)
	return nil
}
