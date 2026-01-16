package handler

import (
	"context"
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
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

// GetTree lấy cây content nodes từ một root node (recursive)
// Endpoint: GET /api/v1/content/nodes/tree/:id
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic đệ quy phức tạp:
//    - Lấy root node từ ID
//    - Query children của root node (sử dụng GetChildren service method)
//    - Đệ quy build tree cho từng child (gọi buildTree đệ quy)
//    - Trả về cấu trúc tree với children nested trong parent
// 2. Query đặc biệt:
//    - Sử dụng service method GetChildren (không phải Find đơn giản)
//    - Cần query đệ quy nhiều lần để lấy toàn bộ tree
// 3. Response format đặc biệt:
//    - Trả về cấu trúc tree (nested structure) thay vì flat array
//    - Mỗi node có field "children" chứa array các child nodes
//    - Format: {id, type, name, text, status, metadata, createdAt, updatedAt, children: [...]}
// 4. Performance optimization:
//    - Có thể optimize bằng cách query tất cả nodes cùng lúc rồi build tree trong memory
//    - Nhưng hiện tại dùng recursive query để đơn giản
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì logic đệ quy phức tạp và response format đặc biệt (tree structure)
//
// Tham số:
//   - id: ID của root node
// Trả về:
//   - Cây content nodes với children được populate đệ quy
func (h *ContentNodeHandler) GetTree(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (tự động validate ObjectID format và convert)
		var params dto.ContentNodeTreeParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID // Đã được validate rồi

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy root node (id đã được validate và convert sang ObjectID format)
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
