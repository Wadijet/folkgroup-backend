package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftContentNodeHandler xử lý các request liên quan đến Draft Content Node (L1-L6)
type DraftContentNodeHandler struct {
	BaseHandler[models.DraftContentNode, dto.DraftContentNodeCreateInput, dto.DraftContentNodeUpdateInput]
	DraftContentNodeService *services.DraftContentNodeService
}

// NewDraftContentNodeHandler tạo mới DraftContentNodeHandler
func NewDraftContentNodeHandler() (*DraftContentNodeHandler, error) {
	draftContentNodeService, err := services.NewDraftContentNodeService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft content node service: %v", err)
	}

	handler := &DraftContentNodeHandler{
		DraftContentNodeService: draftContentNodeService,
	}
	handler.BaseService = handler.DraftContentNodeService.BaseServiceMongoImpl

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

// CommitDraftNode commit draft node → production content node
// Endpoint: POST /api/v1/drafts/nodes/:id/commit
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp (workflow commit):
//    - Đây là action nghiệp vụ (commit), không phải CRUD đơn giản
//    - Copy dữ liệu từ DraftContentNode sang ContentNode
//    - Có thể có side effects: update approval status, send notifications, etc.
//    - Có thể có validation: chỉ commit draft đã được approve
// 2. Cross-collection operation:
//    - Tạo document trong collection content_nodes từ document trong collection draft_content_nodes
//    - Không phải update document trong cùng collection
// 3. Service method đặc biệt:
//    - Sử dụng DraftContentNodeService.CommitDraftNode (logic nghiệp vụ phức tạp)
//    - Service method này xử lý toàn bộ logic commit (copy fields, validate, etc.)
// 4. Response format:
//    - Trả về ContentNode đã được tạo (không phải DraftContentNode)
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là workflow action (commit) với logic nghiệp vụ phức tạp,
//           cross-collection operation, và có thể có side effects
//
// Tham số:
//   - id: ID của draft node cần commit
// Trả về:
//   - ContentNode: Content node đã được tạo từ draft
func (h *DraftContentNodeHandler) CommitDraftNode(c fiber.Ctx) error {
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

		// Commit draft → production
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
