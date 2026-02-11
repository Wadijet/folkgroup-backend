package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/initsvc"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserHandler xử lý các request xác thực và quản lý người dùng
type UserHandler struct {
	*basehdl.BaseHandler[models.User, authdto.UserCreateInput, authdto.UserChangeInfoInput]
	userService     *authsvc.UserService
	roleService     *authsvc.RoleService
	userRoleService *authsvc.UserRoleService
}

// NewUserHandler tạo instance mới của UserHandler
func NewUserHandler() (*UserHandler, error) {
	userService, err := authsvc.NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}
	roleService, err := authsvc.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	userRoleService, err := authsvc.NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}
	baseHandler := basehdl.NewBaseHandler[models.User, authdto.UserCreateInput, authdto.UserChangeInfoInput](userService)
	return &UserHandler{
		BaseHandler:     baseHandler,
		userService:     userService,
		roleService:     roleService,
		userRoleService: userRoleService,
	}, nil
}

// HandleLogout xử lý đăng xuất người dùng
func (h *UserHandler) HandleLogout(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeAuth, "User not authenticated", common.StatusUnauthorized, nil))
		return nil
	}
	var input authdto.UserLogoutInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Invalid user ID", common.StatusBadRequest, err))
		return nil
	}
	err = h.userService.Logout(c.Context(), objID, &input)
	h.HandleResponse(c, nil, err)
	return nil
}

// HandleGetProfile lấy thông tin profile của người dùng
func (h *UserHandler) HandleGetProfile(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeAuth, "User not authenticated", common.StatusUnauthorized, nil))
		return nil
	}
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Invalid user ID", common.StatusBadRequest, err))
		return nil
	}
	user, err := h.userService.BaseServiceMongoImpl.FindOneById(c.Context(), objID)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	user.Password = ""
	user.Salt = ""
	user.Tokens = nil
	h.HandleResponse(c, user, nil)
	return nil
}

// HandleUpdateProfile cập nhật thông tin profile
func (h *UserHandler) HandleUpdateProfile(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeAuth, "User not authenticated", common.StatusUnauthorized, nil))
		return nil
	}
	var input authdto.UserChangeInfoInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Invalid user ID", common.StatusBadRequest, err))
		return nil
	}
	update := &basesvc.UpdateData{Set: map[string]interface{}{"name": input.Name}}
	updatedUser, err := h.userService.BaseServiceMongoImpl.UpdateById(c.Context(), objID, update)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	updatedUser.Password = ""
	updatedUser.Salt = ""
	updatedUser.Tokens = nil
	h.HandleResponse(c, updatedUser, nil)
	return nil
}

// HandleGetUserRoles lấy danh sách role của người dùng kèm organization
func (h *UserHandler) HandleGetUserRoles(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeAuth, "User not authenticated", common.StatusUnauthorized, nil))
		return nil
	}
	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Invalid user ID", common.StatusBadRequest, err))
		return nil
	}
	ctx := c.Context()
	filter := bson.M{"userId": objID}
	userRoles, err := h.userRoleService.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	organizationService, err := authsvc.NewOrganizationService()
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	result := make([]map[string]interface{}, 0, len(userRoles))
	for _, userRole := range userRoles {
		role, err := h.roleService.BaseServiceMongoImpl.FindOneById(ctx, userRole.RoleID)
		if err != nil {
			continue
		}
		if role.OwnerOrganizationID.IsZero() {
			continue
		}
		orgID := role.OwnerOrganizationID
		org, err := organizationService.BaseServiceMongoImpl.FindOneById(ctx, orgID)
		if err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"roleId":               role.ID.Hex(),
			"roleName":             role.Name,
			"ownerOrganizationId":  org.ID.Hex(),
			"organizationName":     org.Name,
			"organizationCode":    org.Code,
			"organizationType":    org.Type,
			"organizationLevel":   org.Level,
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// HandleLoginWithFirebase đăng nhập bằng Firebase ID token
func (h *UserHandler) HandleLoginWithFirebase(c fiber.Ctx) error {
	var input authdto.FirebaseLoginInput
	if err := h.ParseRequestBody(c, &input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	user, err := h.userService.LoginWithFirebase(c.Context(), &input)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	// First user becomes admin: nếu chưa có admin nào, tự động gán quyền Administrator cho user vừa login
	if initSvc, errInit := initsvc.NewInitService(); errInit == nil {
		if hasAdmin, _ := initSvc.HasAnyAdministrator(); !hasAdmin {
			logrus.WithFields(logrus.Fields{"user_id": user.ID.Hex()}).Info("LoginWithFirebase: Tự động set user đầu tiên làm admin")
			if _, errSet := initSvc.SetAdministrator(user.ID); errSet != nil && errSet != common.ErrUserAlreadyAdmin {
				logrus.WithError(errSet).Warn("LoginWithFirebase: Lỗi khi set admin, không fail login")
			}
		}
	}
	user.Password = ""
	user.Salt = ""
	user.Tokens = nil
	h.HandleResponse(c, user, nil)
	return nil
}
