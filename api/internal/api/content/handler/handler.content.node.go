package contenthdl

import (
	"context"
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/utility"

	"github.com/gofiber/fiber/v3"
)

// ContentNodeHandler xử lý các request liên quan đến Content Node (L1-L6)
type ContentNodeHandler struct {
	*basehdl.BaseHandler[contentmodels.ContentNode, contentdto.ContentNodeCreateInput, contentdto.ContentNodeUpdateInput]
	ContentNodeService *contentsvc.ContentNodeService
}

// NewContentNodeHandler tạo mới ContentNodeHandler
func NewContentNodeHandler() (*ContentNodeHandler, error) {
	contentNodeService, err := contentsvc.NewContentNodeService()
	if err != nil {
		return nil, fmt.Errorf("failed to create content node service: %v", err)
	}
	hdl := &ContentNodeHandler{
		ContentNodeService: contentNodeService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.ContentNode, contentdto.ContentNodeCreateInput, contentdto.ContentNodeUpdateInput](contentNodeService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}

// GetTree lấy cây content nodes từ một root node (recursive)
func (h *ContentNodeHandler) GetTree(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params contentdto.ContentNodeTreeParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID

		if err := h.ValidateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		rootID := utility.String2ObjectID(id)
		root, err := h.ContentNodeService.FindOneById(c.Context(), rootID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		tree := h.buildTree(c.Context(), root)
		h.HandleResponse(c, tree, nil)
		return nil
	})
}

func (h *ContentNodeHandler) buildTree(ctx context.Context, node contentmodels.ContentNode) map[string]interface{} {
	result := map[string]interface{}{
		"id": node.ID, "type": node.Type, "name": node.Name, "text": node.Text,
		"status": node.Status, "metadata": node.Metadata,
		"createdAt": node.CreatedAt, "updatedAt": node.UpdatedAt,
	}
	children, err := h.ContentNodeService.GetChildren(ctx, node.ID)
	if err == nil && len(children) > 0 {
		childrenTree := make([]map[string]interface{}, 0, len(children))
		for _, child := range children {
			childrenTree = append(childrenTree, h.buildTree(ctx, child))
		}
		result["children"] = childrenTree
	} else {
		result["children"] = []interface{}{}
	}
	return result
}
