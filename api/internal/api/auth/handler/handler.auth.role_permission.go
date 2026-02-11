package authhdl

import (
	"fmt"
	"time"

	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	models "meta_commerce/internal/api/auth/models"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RolePermissionHandler xử lý các route liên quan đến phân quyền
type RolePermissionHandler struct {
	*basehdl.BaseHandler[models.RolePermission, authdto.RolePermissionCreateInput, models.RolePermission]
	RolePermissionService *authsvc.RolePermissionService
}

// NewRolePermissionHandler tạo instance mới của RolePermissionHandler
func NewRolePermissionHandler() (*RolePermissionHandler, error) {
	rolePermissionService, err := authsvc.NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}
	base := basehdl.NewBaseHandler[models.RolePermission, authdto.RolePermissionCreateInput, models.RolePermission](rolePermissionService)
	return &RolePermissionHandler{
		BaseHandler:            base,
		RolePermissionService: rolePermissionService,
	}, nil
}

// HandleUpdateRolePermissions cập nhật quyền cho vai trò (xóa hết rồi tạo mới)
func (h *RolePermissionHandler) HandleUpdateRolePermissions(c fiber.Ctx) error {
	input := new(authdto.RolePermissionUpdateInput)
	if err := h.ParseRequestBody(c, input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	roleId, err := primitive.ObjectIDFromHex(input.RoleID)
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID vai trò không hợp lệ", common.StatusBadRequest, err))
		return nil
	}
	filter := bson.M{"roleId": roleId}
	if _, err := h.RolePermissionService.BaseServiceMongoImpl.DeleteMany(c.Context(), filter); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	var rolePermissions []models.RolePermission
	now := time.Now().Unix()
	for _, perm := range input.Permissions {
		permissionIdObj, err := primitive.ObjectIDFromHex(perm.PermissionID)
		if err != nil {
			continue
		}
		rolePermissions = append(rolePermissions, models.RolePermission{
			ID:           primitive.NewObjectID(),
			RoleID:       roleId,
			PermissionID: permissionIdObj,
			Scope:        perm.Scope,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	if len(rolePermissions) > 0 {
		_, err = h.RolePermissionService.BaseServiceMongoImpl.InsertMany(c.Context(), rolePermissions)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
	}
	h.HandleResponse(c, rolePermissions, nil)
	return nil
}
