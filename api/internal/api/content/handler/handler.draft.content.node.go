package contenthdl

import (
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
)

// DraftContentNodeHandler xử lý các request liên quan đến Draft Content Node (L1-L6)
type DraftContentNodeHandler struct {
	*basehdl.BaseHandler[contentmodels.DraftContentNode, contentdto.DraftContentNodeCreateInput, contentdto.DraftContentNodeUpdateInput]
	DraftContentNodeService *contentsvc.DraftContentNodeService
}

// NewDraftContentNodeHandler tạo mới DraftContentNodeHandler
func NewDraftContentNodeHandler() (*DraftContentNodeHandler, error) {
	draftContentNodeService, err := contentsvc.NewDraftContentNodeService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft content node service: %v", err)
	}
	hdl := &DraftContentNodeHandler{DraftContentNodeService: draftContentNodeService}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.DraftContentNode, contentdto.DraftContentNodeCreateInput, contentdto.DraftContentNodeUpdateInput](draftContentNodeService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}

// CommitDraftNode commit draft node → production content node
func (h *DraftContentNodeHandler) CommitDraftNode(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params contentdto.CommitDraftNodeParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID
		if err := h.ValidateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		draftID := utility.String2ObjectID(id)
		contentNode, err := h.DraftContentNodeService.CommitDraftNode(c.Context(), draftID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, contentNode, nil)
		return nil
	})
}

// ApproveDraft duyệt một draft
func (h *DraftContentNodeHandler) ApproveDraft(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params contentdto.ApproveDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID
		if err := h.ValidateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		draftID := utility.String2ObjectID(id)
		updated, err := h.DraftContentNodeService.ApproveDraft(c.Context(), draftID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		h.HandleResponse(c, updated, nil)
		return nil
	})
}

// RejectDraft từ chối một draft
func (h *DraftContentNodeHandler) RejectDraft(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params contentdto.RejectDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID
		var input contentdto.RejectDraftInput
		_ = h.ParseRequestBody(c, &input)
		if err := h.ValidateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		draftID := utility.String2ObjectID(id)
		updated, err := h.DraftContentNodeService.RejectDraft(c.Context(), draftID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		if input.DecisionNote != "" {
			metadata := updated.Metadata
			if metadata == nil {
				metadata = make(map[string]interface{})
			}
			metadata["decisionNote"] = input.DecisionNote
			updatedDraft, err := h.DraftContentNodeService.UpdateById(c.Context(), draftID, &basesvc.UpdateData{
				Set: map[string]interface{}{"metadata": metadata},
			})
			if err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
			updated = &updatedDraft
		}
		h.HandleResponse(c, updated, nil)
		return nil
	})
}
