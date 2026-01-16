package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftApprovalHandler xử lý các request liên quan đến Draft Approval
type DraftApprovalHandler struct {
	BaseHandler[models.DraftApproval, dto.DraftApprovalCreateInput, dto.DraftApprovalUpdateInput]
	DraftApprovalService *services.DraftApprovalService
}

// NewDraftApprovalHandler tạo mới DraftApprovalHandler
func NewDraftApprovalHandler() (*DraftApprovalHandler, error) {
	draftApprovalService, err := services.NewDraftApprovalService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft approval service: %v", err)
	}

	handler := &DraftApprovalHandler{
		DraftApprovalService: draftApprovalService,
	}
	handler.BaseService = handler.DraftApprovalService.BaseServiceMongoImpl

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

// InsertOne override method InsertOne để xử lý ownerOrganizationId và gọi service
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseHandler.InsertOne trực tiếp):
// 1. Xử lý ownerOrganizationId:
//    - Cho phép chỉ định từ request hoặc dùng context
//    - Validate quyền nếu có ownerOrganizationId trong request
//    - BaseHandler.InsertOne không tự động xử lý ownerOrganizationId từ request body
//
// LƯU Ý:
// - Validation format đã được xử lý tự động bởi struct tag validate:"required" trong BaseHandler
// - ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
// - Business logic validation (cross-field: ít nhất một target) đã được chuyển xuống DraftApprovalService.InsertOne
// - Business logic (set RequestedBy, RequestedAt, Status) đã được chuyển xuống DraftApprovalService.PrepareForInsert
// - Timestamps sẽ được xử lý tự động bởi BaseServiceMongoImpl.InsertOne trong service
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Parse và validate input format (DTO validation)
// ✅ Transform DTO → Model (transform tags)
// ✅ Xử lý ownerOrganizationId (từ request hoặc context)
// ✅ Gọi DraftApprovalService.InsertOne (service sẽ validate targets, prepare model và insert)
func (h *DraftApprovalHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.DraftApprovalCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Transform DTO sang Model sử dụng transform tag (tự động convert ObjectID)
		model, err := h.transformCreateInputToModel(&input)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi transform dữ liệu: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(model)
		if ownerOrgIDFromRequest != nil && !ownerOrgIDFromRequest.IsZero() {
			// Có ownerOrganizationId trong request → Validate quyền
			if err := h.validateUserHasAccessToOrg(c, *ownerOrgIDFromRequest); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
		} else {
			// Không có trong request → Dùng context
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				h.setOrganizationID(model, *activeOrgID)
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin và set RequestedBy
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		// ✅ Gọi service để insert (service sẽ tự validate targets và prepare model)
		// Business logic validation đã được chuyển xuống DraftApprovalService.InsertOne
		data, err := h.DraftApprovalService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// ApproveDraftWorkflowRun approve tất cả drafts của một workflow run
// Endpoint: POST /api/v1/content/drafts/approvals/:id/approve
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp:
//   - Không chỉ update status, mà còn set decidedBy (từ context), decidedAt (timestamp hiện tại)
//   - Validate status hiện tại phải là "pending" (không cho approve/reject approval đã xử lý)
//   - Trigger logic commit drafts sau khi approve (đã implement trong ApproveDraftWorkflowRun)
//
// 2. Workflow đặc biệt:
//   - Đây là action nghiệp vụ (approve), không phải update đơn giản
//   - Có thể có side effects (commit drafts, send notifications, etc.)
//   - Cần validate quyền đặc biệt (chỉ người có quyền mới được approve)
//
// 3. Response format đặc biệt:
//   - Trả về approval đã được update với thông tin quyết định
//   - Có thể trả về thêm thông tin về drafts đã được commit (khi implement)
//
// Tham số:
//   - id: ID của approval request
//
// Body:
//   - decisionNote: Ghi chú về quyết định (tùy chọn)
func (h *DraftApprovalHandler) ApproveDraftWorkflowRun(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (tự động validate ObjectID format và convert)
		var params dto.ApproveDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID // Đã được validate rồi

		// Parse decision note từ body (tự động validate với struct tag)
		var input dto.ApproveDraftInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			// Không có body cũng OK, chỉ có decisionNote là tùy chọn
			input.DecisionNote = ""
		}

		// Lấy user ID từ context
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok || userIDStr == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeAuthToken,
				"Không tìm thấy user ID trong context",
				common.StatusUnauthorized,
				nil,
			))
			return nil
		}
		userID, err := primitive.ObjectIDFromHex(userIDStr)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("User ID không hợp lệ: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy approval request
		approvalID := utility.String2ObjectID(id)
		approval, err := h.DraftApprovalService.FindOneById(c.Context(), approvalID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Kiểm tra status
		if approval.Status != models.ApprovalRequestStatusPending {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Approval request đã được xử lý (status: %s)", approval.Status),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Update approval status
		updateData := map[string]interface{}{
			"status":       models.ApprovalRequestStatusApproved,
			"decidedBy":    userID,
			"decidedAt":    time.Now().UnixMilli(),
			"decisionNote": input.DecisionNote,
		}
		updatedApproval, err := h.DraftApprovalService.UpdateById(c.Context(), approvalID, &services.UpdateData{
			Set: updateData,
		})
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Nếu có workflowRunID, commit tất cả drafts của workflow run tuần tự theo level
		if approval.WorkflowRunID != nil {
			// Lấy draft content node service
			draftContentNodeService, err := services.NewDraftContentNodeService()
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeInternalServer,
					fmt.Sprintf("Lỗi khi khởi tạo draft content node service: %v", err),
					common.StatusInternalServerError,
					err,
				))
				return nil
			}

			// Lấy tất cả drafts của workflow run
			drafts, err := draftContentNodeService.GetDraftsByWorkflowRunID(c.Context(), *approval.WorkflowRunID)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeDatabaseQuery,
					fmt.Sprintf("Lỗi khi lấy drafts của workflow run: %v", err),
					common.StatusInternalServerError,
					err,
				))
				return nil
			}

			// Sắp xếp drafts theo level (L1 → L2 → ... → L6)
			// Tạo map level → drafts
			levelDrafts := make(map[int][]models.DraftContentNode)
			for _, draft := range drafts {
				level := utility.GetContentLevel(draft.Type)
				if level > 0 {
					levelDrafts[level] = append(levelDrafts[level], draft)
				}
			}

			// Commit tuần tự từ L1 → L2 → ... → L6
			var committedNodes []models.ContentNode
			for level := 1; level <= 6; level++ {
				draftsAtLevel, exists := levelDrafts[level]
				if !exists {
					continue // Không có draft ở level này
				}

				// Commit tất cả drafts ở level này
				for _, draft := range draftsAtLevel {
					// Kiểm tra draft đã được approve chưa
					if draft.ApprovalStatus != models.DraftApprovalStatusApproved {
						// Update approval status của draft
						updateData := map[string]interface{}{
							"approvalStatus": models.DraftApprovalStatusApproved,
						}
						_, err := draftContentNodeService.UpdateById(c.Context(), draft.ID, &services.UpdateData{
							Set: updateData,
						})
						if err != nil {
							h.HandleResponse(c, nil, common.NewError(
								common.ErrCodeDatabaseQuery,
								fmt.Sprintf("Lỗi khi update approval status của draft %s: %v", draft.ID.Hex(), err),
								common.StatusInternalServerError,
								err,
							))
							return nil
						}
						draft.ApprovalStatus = models.DraftApprovalStatusApproved
					}

					// Commit draft → production
					contentNode, err := draftContentNodeService.CommitDraftNode(c.Context(), draft.ID)
					if err != nil {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeBusinessOperation,
							fmt.Sprintf("Lỗi khi commit draft %s (type: %s, L%d): %v. Đã commit thành công %d nodes trước đó.",
								draft.ID.Hex(), draft.Type, level, err, len(committedNodes)),
							common.StatusBadRequest,
							err,
						))
						return nil
					}
					committedNodes = append(committedNodes, *contentNode)
				}
			}

			// Trả về approval với thông tin đã commit
			// Có thể thêm metadata về committed nodes nếu cần
			updatedApproval.Metadata = map[string]interface{}{
				"committedNodesCount": len(committedNodes),
				"committedNodeIds": func() []string {
					ids := make([]string, len(committedNodes))
					for i, node := range committedNodes {
						ids[i] = node.ID.Hex()
					}
					return ids
				}(),
			}
		}

		h.HandleResponse(c, updatedApproval, nil)
		return nil
	})
}

// RejectDraftWorkflowRun reject approval request
// Endpoint: POST /api/v1/content/drafts/approvals/:id/reject
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp:
//   - Không chỉ update status, mà còn set decidedBy (từ context), decidedAt (timestamp hiện tại)
//   - Validate status hiện tại phải là "pending" (không cho reject approval đã xử lý)
//   - DecisionNote là BẮT BUỘC khi reject (validation đặc biệt)
//
// 2. Workflow đặc biệt:
//   - Đây là action nghiệp vụ (reject), không phải update đơn giản
//   - Có thể có side effects (send notifications, update draft status, etc.)
//   - Cần validate quyền đặc biệt (chỉ người có quyền mới được reject)
//
// 3. Validation đặc biệt:
//   - DecisionNote phải có giá trị (bắt buộc khi reject) - khác với approve (optional)
//   - Đây là business rule: khi reject phải có lý do
//
// Tham số:
//   - id: ID của approval request
//
// Body:
//   - decisionNote: Ghi chú về quyết định (bắt buộc khi reject)
func (h *DraftApprovalHandler) RejectDraftWorkflowRun(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse và validate URL params (tự động validate ObjectID format và convert)
		var params dto.RejectDraftParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		id := params.ID // Đã được validate rồi

		// Parse decision note từ body (tự động validate với struct tag - required)
		var input dto.RejectDraftInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy user ID từ context
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok || userIDStr == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeAuthToken,
				"Không tìm thấy user ID trong context",
				common.StatusUnauthorized,
				nil,
			))
			return nil
		}
		userID, err := primitive.ObjectIDFromHex(userIDStr)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("User ID không hợp lệ: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate quyền truy cập
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Lấy approval request
		approvalID := utility.String2ObjectID(id)
		approval, err := h.DraftApprovalService.FindOneById(c.Context(), approvalID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Kiểm tra status
		if approval.Status != models.ApprovalRequestStatusPending {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Approval request đã được xử lý (status: %s)", approval.Status),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Update approval status
		updateData := map[string]interface{}{
			"status":       models.ApprovalRequestStatusRejected,
			"decidedBy":    userID,
			"decidedAt":    time.Now().UnixMilli(),
			"decisionNote": input.DecisionNote,
		}
		updatedApproval, err := h.DraftApprovalService.UpdateById(c.Context(), approvalID, &services.UpdateData{
			Set: updateData,
		})
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		h.HandleResponse(c, updatedApproval, nil)
		return nil
	})
}
