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

// OrganizationConfigItemHandler xử lý CRUD config item và resolved config
type OrganizationConfigItemHandler struct {
	*basehdl.BaseHandler[models.OrganizationConfigItem, authdto.OrganizationConfigItemUpsertInput, authdto.OrganizationConfigItemUpsertInput]
	ItemService *authsvc.OrganizationConfigItemService
}

// NewOrganizationConfigItemHandler tạo handler cho organization config item
func NewOrganizationConfigItemHandler() (*OrganizationConfigItemHandler, error) {
	svc, err := authsvc.NewOrganizationConfigItemService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization config item service: %w", err)
	}
	base := basehdl.NewBaseHandler[models.OrganizationConfigItem, authdto.OrganizationConfigItemUpsertInput, authdto.OrganizationConfigItemUpsertInput](svc)
	return &OrganizationConfigItemHandler{
		BaseHandler: base,
		ItemService: svc,
	}, nil
}

// GetResolved xử lý GET /organization-config/resolved
func (h *OrganizationConfigItemHandler) GetResolved(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		orgIDStr := c.Query("ownerOrganizationId")
		if orgIDStr == "" {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationInput, "Thiếu ownerOrganizationId", common.StatusBadRequest, nil))
			return nil
		}
		orgID, err := primitive.ObjectIDFromHex(orgIDStr)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationInput, "ownerOrganizationId không hợp lệ", common.StatusBadRequest, err))
			return nil
		}
		if err := h.ValidateUserHasAccessToOrg(c, orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		resolved, err := h.ItemService.GetResolvedConfig(c.Context(), orgID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, fiber.Map{"config": resolved}, nil)
		return nil
	})
}
