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

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model
func (h *DraftContentNodeHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.DraftContentNodeCreateInput
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
		draftNode := models.DraftContentNode{
			Type:     input.Type,
			Name:     input.Name,
			Text:     input.Text,
			Metadata: input.Metadata,
		}

		// Xử lý ParentID hoặc ParentDraftID
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
			draftNode.ParentID = &parentID
		}
		if input.ParentDraftID != "" {
			if !primitive.IsValidObjectID(input.ParentDraftID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("ParentDraftID '%s' không đúng định dạng MongoDB ObjectID", input.ParentDraftID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			parentDraftID := utility.String2ObjectID(input.ParentDraftID)
			draftNode.ParentDraftID = &parentDraftID
		}

		// Xử lý workflow run IDs
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
			draftNode.WorkflowRunID = &workflowRunID
		}
		if input.CreatedByRunID != "" {
			if !primitive.IsValidObjectID(input.CreatedByRunID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("CreatedByRunID '%s' không đúng định dạng MongoDB ObjectID", input.CreatedByRunID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			createdByRunID := utility.String2ObjectID(input.CreatedByRunID)
			draftNode.CreatedByRunID = &createdByRunID
		}
		if input.CreatedByStepRunID != "" {
			if !primitive.IsValidObjectID(input.CreatedByStepRunID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("CreatedByStepRunID '%s' không đúng định dạng MongoDB ObjectID", input.CreatedByStepRunID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			createdByStepRunID := utility.String2ObjectID(input.CreatedByStepRunID)
			draftNode.CreatedByStepRunID = &createdByStepRunID
		}
		if input.CreatedByCandidateID != "" {
			if !primitive.IsValidObjectID(input.CreatedByCandidateID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("CreatedByCandidateID '%s' không đúng định dạng MongoDB ObjectID", input.CreatedByCandidateID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			createdByCandidateID := utility.String2ObjectID(input.CreatedByCandidateID)
			draftNode.CreatedByCandidateID = &createdByCandidateID
		}
		if input.CreatedByBatchID != "" {
			if !primitive.IsValidObjectID(input.CreatedByBatchID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("CreatedByBatchID '%s' không đúng định dạng MongoDB ObjectID", input.CreatedByBatchID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			createdByBatchID := utility.String2ObjectID(input.CreatedByBatchID)
			draftNode.CreatedByBatchID = &createdByBatchID
		}

		// Set approval status (mặc định: draft)
		if input.ApprovalStatus == "" {
			draftNode.ApprovalStatus = models.DraftApprovalStatusDraft
		} else {
			draftNode.ApprovalStatus = input.ApprovalStatus
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), draftNode)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// CommitDraftNode commit draft node → production content node
// Endpoint: POST /api/v1/drafts/nodes/:id/commit
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
