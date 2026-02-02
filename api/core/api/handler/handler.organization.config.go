package handler

import (
	"errors"
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationConfigHandler xử lý các request liên quan đến config tổ chức.
// Endpoint: GET/PUT/DELETE /organization/:id/config, GET /organization/:id/config/resolved.
type OrganizationConfigHandler struct {
	BaseHandler[models.OrganizationConfig, dto.OrganizationConfigUpdateInput, dto.OrganizationConfigUpdateInput]
	OrganizationConfigService *services.OrganizationConfigService
}

// NewOrganizationConfigHandler tạo mới OrganizationConfigHandler.
func NewOrganizationConfigHandler() (*OrganizationConfigHandler, error) {
	svc, err := services.NewOrganizationConfigService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization config service: %w", err)
	}

	base := NewBaseHandler[models.OrganizationConfig, dto.OrganizationConfigUpdateInput, dto.OrganizationConfigUpdateInput](svc)
	return &OrganizationConfigHandler{
		BaseHandler:               *base,
		OrganizationConfigService: svc,
	}, nil
}

// GetConfig xử lý GET /organization/:id/config — lấy config raw của tổ chức (chưa merge theo cây).
func (h *OrganizationConfigHandler) GetConfig(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		orgID, err := h.parseOrganizationIDFromParams(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		if err := h.validateUserHasAccessToOrg(c, *orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		doc, err := h.OrganizationConfigService.GetByOwnerOrganizationID(c.Context(), *orgID)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				// Chưa có config → trả về document rỗng (config + configMeta null)
				h.HandleResponse(c, fiber.Map{
					"ownerOrganizationId": orgID.Hex(),
					"config":             nil,
					"configMeta":         nil,
					"isSystem":           false,
				}, nil)
				return nil
			}
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, doc, nil)
		return nil
	})
}

// GetResolvedConfig xử lý GET /organization/:id/config/resolved — lấy config đã merge theo cây (root → org).
func (h *OrganizationConfigHandler) GetResolvedConfig(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		orgID, err := h.parseOrganizationIDFromParams(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		if err := h.validateUserHasAccessToOrg(c, *orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		resolved, err := h.OrganizationConfigService.GetResolvedConfig(c.Context(), *orgID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, fiber.Map{"config": resolved}, nil)
		return nil
	})
}

// UpdateConfig xử lý PUT /organization/:id/config — tạo hoặc cập nhật config (upsert).
// Body: OrganizationConfigUpdateInput (config, configMeta). IsSystem luôn false từ API.
func (h *OrganizationConfigHandler) UpdateConfig(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		orgID, err := h.parseOrganizationIDFromParams(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		if err := h.validateUserHasAccessToOrg(c, *orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		var input dto.OrganizationConfigUpdateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		configMeta := dtoConfigMetaToModel(input.ConfigMeta)
		doc, err := h.OrganizationConfigService.UpsertByOwnerOrganizationID(
			c.Context(), *orgID, input.Config, configMeta, false,
		)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, doc, nil)
		return nil
	})
}

// DeleteConfig xử lý DELETE /organization/:id/config — xóa config (không cho xóa config hệ thống).
func (h *OrganizationConfigHandler) DeleteConfig(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		orgID, err := h.parseOrganizationIDFromParams(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		if err := h.validateUserHasAccessToOrg(c, *orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		if err := h.OrganizationConfigService.DeleteByOwnerOrganizationID(c.Context(), *orgID); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, fiber.Map{"message": "Đã xóa config tổ chức."}, nil)
		return nil
	})
}

// parseOrganizationIDFromParams lấy và parse organization ID từ tham số :id.
func (h *OrganizationConfigHandler) parseOrganizationIDFromParams(c fiber.Ctx) (*primitive.ObjectID, error) {
	idStr := c.Params("id")
	if idStr == "" {
		return nil, common.NewError(common.ErrCodeValidationInput, "Thiếu id tổ chức.", common.StatusBadRequest, nil)
	}
	orgID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, common.NewError(common.ErrCodeValidationInput, "Id tổ chức không hợp lệ.", common.StatusBadRequest, err)
	}
	return &orgID, nil
}

// dtoConfigMetaToModel chuyển ConfigKeyMetaInput (DTO) sang ConfigKeyMeta (model).
func dtoConfigMetaToModel(meta map[string]dto.ConfigKeyMetaInput) map[string]models.ConfigKeyMeta {
	if meta == nil {
		return nil
	}
	out := make(map[string]models.ConfigKeyMeta, len(meta))
	for k, v := range meta {
		out[k] = models.ConfigKeyMeta{
			Name:          v.Name,
			Description:   v.Description,
			DataType:      v.DataType,
			Constraints:   v.Constraints,
			AllowOverride: v.AllowOverride,
		}
	}
	return out
}
