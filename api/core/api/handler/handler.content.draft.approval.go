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

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
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

		// Validate: Phải có ít nhất một target (workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID)
		hasTarget := input.WorkflowRunID != "" || input.DraftNodeID != "" || input.DraftVideoID != "" || input.DraftPublicationID != ""
		if !hasTarget {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"Phải có ít nhất một target: workflowRunId, draftNodeId, draftVideoId, hoặc draftPublicationId",
				common.StatusBadRequest,
				nil,
			))
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

		// Chuyển đổi DTO sang Model
		approval := models.DraftApproval{
			Status:      models.ApprovalRequestStatusPending,
			RequestedBy: userID,
			RequestedAt: time.Now().UnixMilli(),
			Metadata:    input.Metadata,
		}

		// Xử lý các target IDs
		if input.WorkflowRunID != "" {
			if !primitive.IsValidObjectID(input.WorkflowRunID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("WorkflowRunID '%s' không đúng định dạng MongoDB ObjectID", input.WorkflowRunID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			workflowRunID := utility.String2ObjectID(input.WorkflowRunID)
			approval.WorkflowRunID = &workflowRunID
		}
		if input.DraftNodeID != "" {
			if !primitive.IsValidObjectID(input.DraftNodeID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("DraftNodeID '%s' không đúng định dạng MongoDB ObjectID", input.DraftNodeID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			draftNodeID := utility.String2ObjectID(input.DraftNodeID)
			approval.DraftNodeID = &draftNodeID
		}
		if input.DraftVideoID != "" {
			if !primitive.IsValidObjectID(input.DraftVideoID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("DraftVideoID '%s' không đúng định dạng MongoDB ObjectID", input.DraftVideoID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			draftVideoID := utility.String2ObjectID(input.DraftVideoID)
			approval.DraftVideoID = &draftVideoID
		}
		if input.DraftPublicationID != "" {
			if !primitive.IsValidObjectID(input.DraftPublicationID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("DraftPublicationID '%s' không đúng định dạng MongoDB ObjectID", input.DraftPublicationID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			draftPublicationID := utility.String2ObjectID(input.DraftPublicationID)
			approval.DraftPublicationID = &draftPublicationID
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), approval)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// ApproveDraftWorkflowRun approve tất cả drafts của một workflow run
// Endpoint: POST /api/v1/content/drafts/approvals/:id/approve
// Tham số:
//   - id: ID của approval request
// Body:
//   - decisionNote: Ghi chú về quyết định (tùy chọn)
func (h *DraftApprovalHandler) ApproveDraftWorkflowRun(c fiber.Ctx) error {
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

		// Parse decision note từ body
		var body struct {
			DecisionNote string `json:"decisionNote,omitempty"`
		}
		if err := h.ParseRequestBody(c, &body); err != nil {
			// Không có body cũng OK, chỉ có decisionNote là tùy chọn
			body.DecisionNote = ""
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
			"status":     models.ApprovalRequestStatusApproved,
			"decidedBy":  userID,
			"decidedAt":  time.Now().UnixMilli(),
			"decisionNote": body.DecisionNote,
		}
		updatedApproval, err := h.DraftApprovalService.UpdateById(c.Context(), approvalID, &services.UpdateData{
			Set: updateData,
		})
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// TODO: Nếu có workflowRunID, commit tất cả drafts của workflow run
		// Logic này sẽ được implement sau khi có service commit drafts

		h.HandleResponse(c, updatedApproval, nil)
		return nil
	})
}

// RejectDraftWorkflowRun reject approval request
// Endpoint: POST /api/v1/content/drafts/approvals/:id/reject
// Tham số:
//   - id: ID của approval request
// Body:
//   - decisionNote: Ghi chú về quyết định (bắt buộc khi reject)
func (h *DraftApprovalHandler) RejectDraftWorkflowRun(c fiber.Ctx) error {
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

		// Parse decision note từ body
		var body struct {
			DecisionNote string `json:"decisionNote" validate:"required"`
		}
		if err := h.ParseRequestBody(c, &body); err != nil || body.DecisionNote == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"decisionNote là bắt buộc khi reject",
				common.StatusBadRequest,
				err,
			))
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
			"status":      models.ApprovalRequestStatusRejected,
			"decidedBy":   userID,
			"decidedAt":   time.Now().UnixMilli(),
			"decisionNote": body.DecisionNote,
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
