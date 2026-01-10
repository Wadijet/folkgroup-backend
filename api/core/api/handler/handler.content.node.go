package handler

import (
	"context"
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ContentNodeHandler xử lý các request liên quan đến Content Node (L1-L6)
type ContentNodeHandler struct {
	BaseHandler[models.ContentNode, dto.ContentNodeCreateInput, dto.ContentNodeUpdateInput]
	ContentNodeService *services.ContentNodeService
}

// NewContentNodeHandler tạo mới ContentNodeHandler
func NewContentNodeHandler() (*ContentNodeHandler, error) {
	contentNodeService, err := services.NewContentNodeService()
	if err != nil {
		return nil, fmt.Errorf("failed to create content node service: %v", err)
	}

	handler := &ContentNodeHandler{
		ContentNodeService: contentNodeService,
	}
	handler.BaseService = handler.ContentNodeService.BaseServiceMongoImpl

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{
			"password",
			"token",
			"secret",
			"key",
			"hash",
		},
		AllowedOperators: []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		},
		MaxFields: 10,
	}

	return handler, nil
}

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
func (h *ContentNodeHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.ContentNodeCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate type
		validTypes := []string{
			models.ContentNodeTypeLayer,
			models.ContentNodeTypeSTP,
			models.ContentNodeTypeInsight,
			models.ContentNodeTypeContentLine,
			models.ContentNodeTypeGene,
			models.ContentNodeTypeScript,
		}
		typeValid := false
		for _, validType := range validTypes {
			if input.Type == validType {
				typeValid = true
				break
			}
		}
		if !typeValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Type '%s' không hợp lệ. Các giá trị hợp lệ: %v", input.Type, validTypes),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		contentNode := models.ContentNode{
			Type:     input.Type,
			Name:     input.Name,
			Text:     input.Text,
			Metadata: input.Metadata,
		}

		// Xử lý ParentID nếu có
		if input.ParentID != "" {
			if !primitive.IsValidObjectID(input.ParentID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("ParentID '%s' không đúng định dạng MongoDB ObjectID", input.ParentID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			parentID := utility.String2ObjectID(input.ParentID)
			contentNode.ParentID = &parentID
		}

		// Set creator type và creation method (mặc định: human, manual)
		if input.CreatorType == "" {
			contentNode.CreatorType = models.CreatorTypeHuman
		} else {
			contentNode.CreatorType = input.CreatorType
		}
		if input.CreationMethod == "" {
			contentNode.CreationMethod = models.CreationMethodManual
		} else {
			contentNode.CreationMethod = input.CreationMethod
		}

		// Set status (mặc định: active)
		if input.Status == "" {
			contentNode.Status = "active"
		} else {
			contentNode.Status = input.Status
		}

		// Thực hiện insert
		ctx := c.Context()
		data, err := h.BaseService.InsertOne(ctx, contentNode)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// GetTree lấy cây content nodes từ một root node (recursive)
// Endpoint: GET /api/v1/content/nodes/tree/:id
// Tham số:
//   - id: ID của root node
// Trả về:
//   - Cây content nodes với children được populate đệ quy
func (h *ContentNodeHandler) GetTree(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		id := c.Params("id")
		if id == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"ID không được để trống trong URL params",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		if !primitive.IsValidObjectID(id) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ID '%s' không đúng định dạng MongoDB ObjectID", id),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy root node
		rootID := utility.String2ObjectID(id)
		root, err := h.ContentNodeService.FindOneById(c.Context(), rootID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Build tree đệ quy
		tree := h.buildTree(c.Context(), root)

		h.HandleResponse(c, tree, nil)
		return nil
	})
}

// buildTree xây dựng cây content nodes đệ quy
func (h *ContentNodeHandler) buildTree(ctx context.Context, node models.ContentNode) map[string]interface{} {
	result := map[string]interface{}{
		"id":       node.ID,
		"type":     node.Type,
		"name":     node.Name,
		"text":     node.Text,
		"status":   node.Status,
		"metadata": node.Metadata,
		"createdAt": node.CreatedAt,
		"updatedAt": node.UpdatedAt,
	}

	// Lấy children
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
