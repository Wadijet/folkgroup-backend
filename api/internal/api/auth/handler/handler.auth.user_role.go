package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	models "meta_commerce/internal/api/auth/models"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRoleHandler xử lý các route liên quan đến vai trò của người dùng
type UserRoleHandler struct {
	*basehdl.BaseHandler[models.UserRole, authdto.UserRoleCreateInput, authdto.UserRoleCreateInput]
	UserRoleService *authsvc.UserRoleService
}

// NewUserRoleHandler tạo instance mới của UserRoleHandler
func NewUserRoleHandler() (*UserRoleHandler, error) {
	userRoleService, err := authsvc.NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}
	base := basehdl.NewBaseHandler[models.UserRole, authdto.UserRoleCreateInput, authdto.UserRoleCreateInput](userRoleService)
	return &UserRoleHandler{
		BaseHandler:     base,
		UserRoleService: userRoleService,
	}, nil
}

// HandleUpdateUserRoles cập nhật vai trò cho người dùng
func (h *UserRoleHandler) HandleUpdateUserRoles(c fiber.Ctx) error {
	input := new(authdto.UserRoleUpdateInput)
	if err := h.ParseRequestBody(c, input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	userId, err := primitive.ObjectIDFromHex(input.UserID)
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID người dùng không hợp lệ", common.StatusBadRequest, err))
		return nil
	}
	var newRoleIDs []primitive.ObjectID
	for _, roleIdStr := range input.RoleIDs {
		roleIdObj, err := primitive.ObjectIDFromHex(roleIdStr)
		if err == nil {
			newRoleIDs = append(newRoleIDs, roleIdObj)
		}
	}
	userRoles, err := h.UserRoleService.UpdateUserRoles(c.Context(), userId, newRoleIDs)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	h.HandleResponse(c, userRoles, nil)
	return nil
}
