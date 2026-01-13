package handler

import (
	"fmt"
	"time"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowRunHandler xử lý các request liên quan đến AI Workflow Run (Module 2)
type AIWorkflowRunHandler struct {
	*BaseHandler[models.AIWorkflowRun, dto.AIWorkflowRunCreateInput, dto.AIWorkflowRunUpdateInput]
	AIWorkflowRunService *services.AIWorkflowRunService
}

// NewAIWorkflowRunHandler tạo mới AIWorkflowRunHandler
// Trả về:
//   - *AIWorkflowRunHandler: Instance mới của AIWorkflowRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowRunHandler() (*AIWorkflowRunHandler, error) {
	aiWorkflowRunService, err := services.NewAIWorkflowRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow run service: %v", err)
	}

	handler := &AIWorkflowRunHandler{
		AIWorkflowRunService: aiWorkflowRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflowRun, dto.AIWorkflowRunCreateInput, dto.AIWorkflowRunUpdateInput](aiWorkflowRunService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override method InsertOne để set default values
//
// LÝ DO PHẢI OVERRIDE:
// 1. Set default Status = "pending"
// 2. Set default CurrentStepIndex = 0
// 3. Set default StepRunIDs = []
// 4. Set CreatedAt tự động (timestamp milliseconds)
//
// LƯU Ý: ObjectID conversion đã được xử lý tự động bởi transform tag trong DTO
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowRunCreateInput
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

		// Set default values
		now := time.Now().UnixMilli()
		model.Status = models.AIWorkflowRunStatusPending // Mặc định
		model.CurrentStepIndex = 0
		model.StepRunIDs = []primitive.ObjectID{}
		model.CreatedAt = now

		// ✅ Xử lý ownerOrganizationId: Lấy từ role context (giống BaseHandler nhưng cần set trước khi gọi)
		if activeRoleIDStr, ok := c.Locals("active_role_id").(string); ok && activeRoleIDStr != "" {
			if activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr); err == nil {
				roleService, err := services.NewRoleService()
				if err == nil {
					if role, err := roleService.FindOneById(c.Context(), activeRoleID); err == nil {
						if !role.OwnerOrganizationID.IsZero() {
							model.OwnerOrganizationID = role.OwnerOrganizationID
						}
					}
				}
			}
		}

		// Fallback: Nếu vẫn chưa có, thử lấy từ active_organization_id trong context
		if model.OwnerOrganizationID.IsZero() {
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				model.OwnerOrganizationID = *activeOrgID
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		// ✅ Validate RootRefID nếu có: Kiểm tra rootRefID phải tồn tại và đúng level
		if model.RootRefID != nil && model.RootRefType != "" {
			// Lấy content node service và draft content node service để kiểm tra
			contentNodeService, err := services.NewContentNodeService()
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeInternalServer,
					fmt.Sprintf("Lỗi khi khởi tạo content node service: %v", err),
					common.StatusInternalServerError,
					err,
				))
				return nil
			}

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

			// Kiểm tra rootRefID tồn tại và đúng type
			var rootType string
			var rootExists bool
			var rootIsProduction bool
			var rootIsApproved bool

			// Thử tìm trong production trước
			rootProduction, err := contentNodeService.FindOneById(ctx, *model.RootRefID)
			if err == nil {
				// Root tồn tại trong production
				rootType = rootProduction.Type
				rootExists = true
				rootIsProduction = true
				rootIsApproved = true // Production = đã approve
			} else if err == common.ErrNotFound {
				// Không tìm thấy trong production, thử tìm trong draft
				rootDraft, err := draftContentNodeService.FindOneById(ctx, *model.RootRefID)
				if err == nil {
					// Root tồn tại trong draft
					rootType = rootDraft.Type
					rootExists = true
					rootIsProduction = false
					rootIsApproved = (rootDraft.ApprovalStatus == models.DraftApprovalStatusApproved)
				} else if err == common.ErrNotFound {
					// Root không tồn tại
					rootExists = false
				} else {
					h.HandleResponse(c, nil, err)
					return nil
				}
			} else {
				h.HandleResponse(c, nil, err)
				return nil
			}

			// Kiểm tra rootRefID tồn tại
			if !rootExists {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("RootRefID '%s' không tồn tại trong production hoặc draft", model.RootRefID.Hex()),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}

			// Kiểm tra RootRefType đúng với type của rootRefID
			if rootType != model.RootRefType {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("RootRefType '%s' không khớp với type của RootRefID. RootRefID có type: '%s'", model.RootRefType, rootType),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}

			// Kiểm tra rootRefID đã được commit (production) hoặc là draft đã được approve
			// Đây là validation để đảm bảo workflow chỉ bắt đầu từ content đã sẵn sàng
			if !rootIsProduction {
				// Root là draft, phải đã được approve
				if !rootIsApproved {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("RootRefID '%s' (type: %s) là draft chưa được approve. Phải approve và commit root trước khi bắt đầu workflow",
							model.RootRefID.Hex(), rootType),
						common.StatusBadRequest,
						nil,
					))
					return nil
				}
			}

			// Validate sequential level constraint: RootRefType phải hợp lệ
			rootLevel := utility.GetContentLevel(rootType)
			if rootLevel == 0 {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("RootRefType '%s' không hợp lệ. Các type hợp lệ: layer, stp, insight, contentLine, gene, script", rootType),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}
