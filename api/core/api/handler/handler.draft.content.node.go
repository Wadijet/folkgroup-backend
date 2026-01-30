package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
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
		// Parse và validate URL params (tự động validate ObjectID format và convert)
		var params dto.CommitDraftNodeParams
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

// ApproveDraft duyệt một draft (set approvalStatus = approved).
// Endpoint: POST /api/v1/content/drafts/nodes/:id/approve
//
// Validation:
//   - Chỉ approve khi status = pending hoặc draft
//   - Không tự động commit, user phải gọi commit riêng
func (h *DraftContentNodeHandler) ApproveDraft(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params dto.ApproveDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Approve draft
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

// RejectDraft từ chối một draft (set approvalStatus = rejected).
// Endpoint: POST /api/v1/content/drafts/nodes/:id/reject
//
// Validation:
//   - Chỉ reject khi status = pending hoặc draft
func (h *DraftContentNodeHandler) RejectDraft(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params dto.RejectDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID

		// Body (decisionNote) tùy chọn
		var input dto.RejectDraftInput
		_ = h.ParseRequestBody(c, &input)

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Reject draft (service method sẽ validate status)
		draftID := utility.String2ObjectID(id)
		updated, err := h.DraftContentNodeService.RejectDraft(c.Context(), draftID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Nếu có decisionNote, update metadata
		if input.DecisionNote != "" {
			metadata := updated.Metadata
			if metadata == nil {
				metadata = make(map[string]interface{})
			}
			metadata["decisionNote"] = input.DecisionNote
			updatedDraft, err := h.DraftContentNodeService.UpdateById(c.Context(), draftID, &services.UpdateData{
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
