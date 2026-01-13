package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowCommandHandler xử lý các request liên quan đến AI Workflow Command (Module 2)
type AIWorkflowCommandHandler struct {
	*BaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput]
	AIWorkflowCommandService *services.AIWorkflowCommandService
}

// NewAIWorkflowCommandHandler tạo mới AIWorkflowCommandHandler
// Trả về:
//   - *AIWorkflowCommandHandler: Instance mới của AIWorkflowCommandHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowCommandHandler() (*AIWorkflowCommandHandler, error) {
	aiWorkflowCommandService, err := services.NewAIWorkflowCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow command service: %v", err)
	}

	handler := &AIWorkflowCommandHandler{
		AIWorkflowCommandService: aiWorkflowCommandService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIWorkflowCommand, dto.AIWorkflowCommandCreateInput, dto.AIWorkflowCommandUpdateInput](aiWorkflowCommandService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne override để thêm validation RootRefID
// Kiểm tra RootRefID phải tồn tại, đúng type, và đã được commit hoặc approve
func (h *AIWorkflowCommandHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCommandCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Validate CommandType
		if input.CommandType != models.AIWorkflowCommandTypeStartWorkflow && input.CommandType != models.AIWorkflowCommandTypeExecuteStep {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("CommandType '%s' không hợp lệ. Các loại hợp lệ: %s, %s",
					input.CommandType, models.AIWorkflowCommandTypeStartWorkflow, models.AIWorkflowCommandTypeExecuteStep),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Validate WorkflowID hoặc StepID dựa trên CommandType
		if input.CommandType == models.AIWorkflowCommandTypeStartWorkflow {
			if input.WorkflowID == "" {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"WorkflowID là bắt buộc khi CommandType = START_WORKFLOW",
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
		} else if input.CommandType == models.AIWorkflowCommandTypeExecuteStep {
			if input.StepID == "" {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"StepID là bắt buộc khi CommandType = EXECUTE_STEP",
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
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

		// ✅ Validate StepID và ParentLevel nếu CommandType = EXECUTE_STEP
		if model.CommandType == models.AIWorkflowCommandTypeExecuteStep && model.StepID != nil {
			stepService, err := services.NewAIStepService()
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeInternalServer,
					fmt.Sprintf("Lỗi khi khởi tạo AI step service: %v", err),
					common.StatusInternalServerError,
					err,
				))
				return nil
			}

			step, err := stepService.FindOneById(c.Context(), *model.StepID)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("StepID '%s' không tồn tại", model.StepID.Hex()),
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			// Kiểm tra Step có ParentLevel match với RootRefType không
			if step.ParentLevel != "" && model.RootRefType != "" {
				// Convert RootRefType sang level number
				rootLevel := utility.GetContentLevel(model.RootRefType)
				
				if rootLevel == 0 {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeValidationFormat,
						fmt.Sprintf("RootRefType '%s' không hợp lệ", model.RootRefType),
						common.StatusBadRequest,
						nil,
					))
					return nil
				}

				// Convert ParentLevel string (ví dụ: "L1", "L2") sang level number
				// ParentLevel format: "L1", "L2", "L3", etc.
				var expectedParentLevelNum int
				if len(step.ParentLevel) >= 2 && step.ParentLevel[0] == 'L' {
					// Parse "L1" -> 1, "L2" -> 2, etc.
					if _, err := fmt.Sscanf(step.ParentLevel, "L%d", &expectedParentLevelNum); err != nil {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeValidationFormat,
							fmt.Sprintf("ParentLevel '%s' của step không hợp lệ. Format đúng: L1, L2, L3, etc.", step.ParentLevel),
							common.StatusBadRequest,
							nil,
						))
						return nil
					}
				} else {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeValidationFormat,
						fmt.Sprintf("ParentLevel '%s' của step không hợp lệ. Format đúng: L1, L2, L3, etc.", step.ParentLevel),
						common.StatusBadRequest,
						nil,
					))
					return nil
				}

				// Kiểm tra RootRefType có match với ParentLevel của step không
				if rootLevel != expectedParentLevelNum {
					// Tìm content type tương ứng với expectedParentLevelNum
					var expectedParentType string
					for contentType, levelNum := range utility.ContentLevelMap {
						if levelNum == expectedParentLevelNum {
							expectedParentType = contentType
							break
						}
					}

					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("RootRefType '%s' (L%d) không khớp với ParentLevel của step '%s' (ParentLevel: %s, L%d). Step này chỉ có thể chạy với parent là %s (L%d)",
							model.RootRefType, rootLevel,
							step.Name, step.ParentLevel, expectedParentLevelNum,
							expectedParentType, expectedParentLevelNum),
						common.StatusBadRequest,
						nil,
					))
					return nil
				}
			}
		}

		// ✅ Validate RootRefID: Kiểm tra rootRefID phải tồn tại và đúng level
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
			rootProduction, err := contentNodeService.FindOneById(c.Context(), *model.RootRefID)
			if err == nil {
				// Root tồn tại trong production
				rootType = rootProduction.Type
				rootExists = true
				rootIsProduction = true
				rootIsApproved = true // Production = đã approve
			} else if err == common.ErrNotFound {
				// Không tìm thấy trong production, thử tìm trong draft
				rootDraft, err := draftContentNodeService.FindOneById(c.Context(), *model.RootRefID)
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
			// Đây là validation để đảm bảo workflow command chỉ bắt đầu từ content đã sẵn sàng
			if !rootIsProduction {
				// Root là draft, phải đã được approve
				if !rootIsApproved {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("RootRefID '%s' (type: %s) là draft chưa được approve. Phải approve và commit root trước khi tạo workflow command",
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

		// Gọi InsertOne của base handler
		return h.BaseHandler.InsertOne(c)
	})
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
// Endpoint: POST /api/v1/ai/workflow-commands/claim-pending
// Body: { "agentId": "agent-123", "limit": 5 }
//
// Đảm bảo các job khác không lấy lại commands đã được claim cho đến khi được giải phóng
func (h *AIWorkflowCommandHandler) ClaimPendingCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCommandClaimInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate limit
		if input.Limit < 1 {
			input.Limit = 1 // Mặc định claim 1 command
		}
		if input.Limit > 100 {
			input.Limit = 100 // Tối đa 100 commands
		}

		// Validate agentId
		if input.AgentID == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"agentId là bắt buộc và không được để trống",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Gọi service để claim commands
		claimedCommands, err := h.AIWorkflowCommandService.ClaimPendingCommands(c.Context(), input.AgentID, input.Limit)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi claim commands: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		// Trả về danh sách commands đã claim (có thể rỗng nếu không có command pending)
		// Đây là trường hợp hợp lệ, không phải lỗi
		h.HandleResponse(c, claimedCommands, nil)
		return nil
	})
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
// Endpoint: POST /api/v1/ai/workflow-commands/update-heartbeat
// Body: { "commandId": "...", "progress": {...} }
//
// Agent phải gọi endpoint này định kỳ để server biết job đang được thực hiện
func (h *AIWorkflowCommandHandler) UpdateHeartbeat(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.AIWorkflowCommandHeartbeatInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Lấy commandId từ URL params hoặc body
		var commandID primitive.ObjectID
		commandIDStr := c.Params("commandId", "")
		if commandIDStr != "" {
			// Lấy từ URL params
			var err error
			commandID, err = primitive.ObjectIDFromHex(commandIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("commandId từ URL không hợp lệ: %v", err),
					common.StatusBadRequest,
					err,
				))
				return nil
			}
		} else {
			// Lấy từ body - CommandID đã được transform thành *primitive.ObjectID
			// Nhưng vì DTO có transform tag, cần parse lại từ JSON gốc
			// Tạm thời lấy từ body raw và parse
			body := c.Body()
			var bodyMap map[string]interface{}
			if err := json.Unmarshal(body, &bodyMap); err == nil {
				if cmdIDStr, ok := bodyMap["commandId"].(string); ok && cmdIDStr != "" {
					var err error
					commandID, err = primitive.ObjectIDFromHex(cmdIDStr)
					if err != nil {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeValidationFormat,
							fmt.Sprintf("commandId từ body không hợp lệ: %v", err),
							common.StatusBadRequest,
							err,
						))
						return nil
					}
				}
			}
			
			if commandID.IsZero() {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"commandId là bắt buộc (có thể truyền qua URL params :commandId hoặc body JSON {\"commandId\": \"...\"})",
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
		}

		// Lấy agentId từ request (có thể từ body hoặc từ context nếu có middleware set)
		// Tạm thời lấy từ query parameter hoặc header, sau này có thể dùng middleware
		agentId := c.Query("agentId", "")
		if agentId == "" {
			// Thử lấy từ header
			agentId = c.Get("X-Agent-ID", "")
		}
		if agentId == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"agentId là bắt buộc (có thể truyền qua query parameter ?agentId=... hoặc header X-Agent-ID)",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Gọi service để update heartbeat
		updatedCommand, err := h.AIWorkflowCommandService.UpdateHeartbeat(c.Context(), commandID, agentId, input.Progress)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Lỗi khi update heartbeat: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		h.HandleResponse(c, updatedCommand, nil)
		return nil
	})
}

// ReleaseStuckCommands giải phóng các commands bị stuck (quá lâu không có heartbeat)
// Endpoint: POST /api/v1/ai/workflow-commands/release-stuck
// Query: ?timeoutSeconds=300 (tùy chọn, mặc định 300 giây = 5 phút)
//
// Method này nên được gọi định kỳ bởi background job hoặc admin
func (h *AIWorkflowCommandHandler) ReleaseStuckCommands(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse timeout từ query parameter
		timeoutSecondsStr := c.Query("timeoutSeconds", "300")
		timeoutSeconds, err := strconv.ParseInt(timeoutSecondsStr, 10, 64)
		if err != nil || timeoutSeconds < 60 {
			timeoutSeconds = 300 // Mặc định 5 phút, tối thiểu 60 giây
		}

		// Gọi service để release stuck commands
		releasedCount, err := h.AIWorkflowCommandService.ReleaseStuckCommands(c.Context(), timeoutSeconds)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi release stuck commands: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		h.HandleResponse(c, map[string]interface{}{
			"releasedCount":   releasedCount,
			"timeoutSeconds":  timeoutSeconds,
			"message":         fmt.Sprintf("Đã giải phóng %d commands bị stuck", releasedCount),
		}, nil)
		return nil
	})
}
