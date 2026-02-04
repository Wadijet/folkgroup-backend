package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationConfigItemHandler xử lý CRUD config item (1 document per key) và resolved config.
// Resource: /organization-config. Routes: find-one, find, upsert-one, delete-one, resolved.
type OrganizationConfigItemHandler struct {
	BaseHandler[models.OrganizationConfigItem, dto.OrganizationConfigItemUpsertInput, dto.OrganizationConfigItemUpsertInput]
	ItemService *services.OrganizationConfigItemService
}

// NewOrganizationConfigItemHandler tạo handler cho organization config item.
func NewOrganizationConfigItemHandler() (*OrganizationConfigItemHandler, error) {
	svc, err := services.NewOrganizationConfigItemService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization config item service: %w", err)
	}
	base := NewBaseHandler[models.OrganizationConfigItem, dto.OrganizationConfigItemUpsertInput, dto.OrganizationConfigItemUpsertInput](svc)
	return &OrganizationConfigItemHandler{
		BaseHandler: *base,
		ItemService: svc,
	}, nil
}

// GetResolved xử lý GET /organization-config/resolved?ownerOrganizationId=...
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
		if err := h.validateUserHasAccessToOrg(c, orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		svc := h.ItemService
		resolved, err := svc.GetResolvedConfig(c.Context(), orgID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, fiber.Map{"config": resolved}, nil)
		return nil
	})
}
