// Package authhdl - handler init (set admin, init org, permissions, roles).
package authhdl

import (
	"fmt"

	authsvc "meta_commerce/internal/api/auth/service"
	ctasvc "meta_commerce/internal/api/cta/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/initsvc"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
)

// InitHandler xử lý các route khởi tạo hệ thống
type InitHandler struct {
	*basehdl.BaseHandler[interface{}, interface{}, interface{}]
	userCRUD       *authsvc.UserService
	permissionCRUD *authsvc.PermissionService
	roleCRUD       *authsvc.RoleService
	initService    *initsvc.InitService
}

// NewInitHandler tạo một instance mới của InitHandler
func NewInitHandler() (*InitHandler, error) {
	h := &InitHandler{}
	h.BaseHandler = &basehdl.BaseHandler[interface{}, interface{}, interface{}]{}

	userService, err := authsvc.NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}
	h.userCRUD = userService
	permissionService, err := authsvc.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	h.permissionCRUD = permissionService
	roleService, err := authsvc.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	h.roleCRUD = roleService
	initService, err := initsvc.NewInitService()
	if err != nil {
		return nil, fmt.Errorf("failed to create init service: %v", err)
	}
	if ctaSvc, errCTA := ctasvc.NewCTALibraryService(); errCTA == nil {
		initService.SetCTALibraryService(ctaSvc)
	}
	h.initService = initService
	h.BaseService = nil
	return h, nil
}

// HandleSetAdministrator thiết lập administrator (chỉ khi chưa có admin)
func (h *InitHandler) HandleSetAdministrator(c fiber.Ctx) error {
	hasAdmin, err := h.initService.HasAnyAdministrator()
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeInternalServer, "Không thể kiểm tra trạng thái admin", common.StatusInternalServerError, err))
		return nil
	}
	if hasAdmin {
		h.HandleResponse(c, nil, common.NewError(
			common.ErrCodeBusinessState,
			"Hệ thống đã có admin. Vui lòng sử dụng endpoint /admin/user/set-administrator/:id với quyền Init.SetAdmin.",
			common.StatusForbidden, nil))
		return nil
	}
	id := h.GetIDFromContext(c)
	if id == "" {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID không hợp lệ", common.StatusBadRequest, nil))
		return nil
	}
	result, err := h.initService.SetAdministrator(utility.String2ObjectID(id))
	h.HandleResponse(c, result, err)
	return nil
}

// HandleInitOrganization khởi tạo Organization Root
func (h *InitHandler) HandleInitOrganization(c fiber.Ctx) error {
	err := h.initService.InitRootOrganization()
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, map[string]string{"message": "Organization Root đã được khởi tạo thành công"}, nil)
	return nil
}

// HandleInitPermissions khởi tạo Permissions
func (h *InitHandler) HandleInitPermissions(c fiber.Ctx) error {
	err := h.initService.InitPermission()
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, map[string]string{"message": "Permissions đã được khởi tạo thành công"}, nil)
	return nil
}

// HandleInitRoles khởi tạo Roles
func (h *InitHandler) HandleInitRoles(c fiber.Ctx) error {
	err := h.initService.InitRole()
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if err := h.initService.CheckPermissionForAdministrator(); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, map[string]string{"message": "Roles đã được khởi tạo thành công"}, nil)
	return nil
}

// HandleInitAdminUser khởi tạo Admin User từ Firebase UID
func (h *InitHandler) HandleInitAdminUser(c fiber.Ctx) error {
	type InitAdminUserInput struct {
		FirebaseUID string `json:"firebaseUid" validate:"required"`
	}
	var input InitAdminUserInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if err := h.initService.InitAdminUser(input.FirebaseUID); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, map[string]string{"message": "Admin user đã được khởi tạo thành công"}, nil)
	return nil
}

// HandleInitAll khởi tạo tất cả (Organization, Permissions, Roles)
func (h *InitHandler) HandleInitAll(c fiber.Ctx) error {
	results := make(map[string]interface{})
	if err := h.initService.InitRootOrganization(); err != nil {
		results["organization"] = map[string]string{"status": "failed", "error": err.Error()}
	} else {
		results["organization"] = map[string]string{"status": "success"}
	}
	if err := h.initService.InitPermission(); err != nil {
		results["permissions"] = map[string]string{"status": "failed", "error": err.Error()}
	} else {
		results["permissions"] = map[string]string{"status": "success"}
	}
	if err := h.initService.InitRole(); err != nil {
		results["roles"] = map[string]string{"status": "failed", "error": err.Error()}
	} else {
		results["roles"] = map[string]string{"status": "success"}
		_ = h.initService.CheckPermissionForAdministrator()
	}
	h.HandleResponse(c, results, nil)
	return nil
}

// HandleInitStatus kiểm tra trạng thái khởi tạo hệ thống
func (h *InitHandler) HandleInitStatus(c fiber.Ctx) error {
	status, err := h.initService.GetInitStatus()
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, status, nil)
	return nil
}
