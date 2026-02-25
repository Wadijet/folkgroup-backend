// Package crmhdl - Handler ghi chú khách hàng CRM.
package crmhdl

import (
	"errors"
	"fmt"
	"strconv"

	basehdl "meta_commerce/internal/api/base/handler"
	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmNoteHandler xử lý CRUD ghi chú khách.
type CrmNoteHandler struct {
	NoteService   *crmvc.CrmNoteService
	ActivitySvc   *crmvc.CrmActivityService
}

// NewCrmNoteHandler tạo CrmNoteHandler mới.
func NewCrmNoteHandler() (*CrmNoteHandler, error) {
	noteSvc, err := crmvc.NewCrmNoteService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmNoteService: %w", err)
	}
	activitySvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmActivityService: %w", err)
	}
	return &CrmNoteHandler{NoteService: noteSvc, ActivitySvc: activitySvc}, nil
}

// HandleCreateNote xử lý POST /customers/:unifiedId/notes.
func (h *CrmNoteHandler) HandleCreateNote(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		unifiedId := c.Params("unifiedId")
		if unifiedId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu unifiedId", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
			})
			return nil
		}
		userID := getUserIDFromContext(c)
		if userID == nil {
			c.Status(common.StatusUnauthorized).JSON(fiber.Map{
				"code": common.ErrCodeAuthToken.Code, "message": "Chưa đăng nhập", "status": "error",
			})
			return nil
		}
		var input crmdto.CrmNoteCreateInput
		if err := c.Bind().Body(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		input.CustomerId = unifiedId
		if input.NoteText == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "noteText không được để trống", "status": "error",
			})
			return nil
		}
		note, err := h.NoteService.CreateNote(c.Context(), &input, *orgID, *userID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi tạo ghi chú", "status": "error",
			})
			return nil
		}
		// LogActivity được gọi tự động qua hook OnDataChanged khi insert crm_notes
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thêm ghi chú thành công", "data": toNoteResponse(note), "status": "success",
		})
		return nil
	})
}

// HandleListNotes xử lý GET /customers/:unifiedId/notes.
func (h *CrmNoteHandler) HandleListNotes(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		unifiedId := c.Params("unifiedId")
		if unifiedId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu unifiedId", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
			})
			return nil
		}
		limit := 50
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		notes, err := h.NoteService.FindByCustomerId(c.Context(), unifiedId, *orgID, limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn ghi chú", "status": "error",
			})
			return nil
		}
		var data []crmdto.CrmNoteResponse
		for _, n := range notes {
			data = append(data, *toNoteResponse(&n))
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": data, "status": "success",
		})
		return nil
	})
}

// HandleDeleteNote xử lý DELETE /customers/:unifiedId/notes/:noteId (soft delete).
func (h *CrmNoteHandler) HandleDeleteNote(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		noteIdStr := c.Params("noteId")
		if noteIdStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu noteId", "status": "error",
			})
			return nil
		}
		noteId, err := primitive.ObjectIDFromHex(noteIdStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "noteId không hợp lệ", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
			})
			return nil
		}
		if err := h.NoteService.SoftDelete(c.Context(), noteId, *orgID); err != nil {
			if errors.Is(err, common.ErrNotFound) {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy ghi chú", "status": "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi xóa ghi chú", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Xóa ghi chú thành công", "data": nil, "status": "success",
		})
		return nil
	})
}

func toNoteResponse(n *crmmodels.CrmNote) *crmdto.CrmNoteResponse {
	if n == nil {
		return nil
	}
	return &crmdto.CrmNoteResponse{
		ID:             n.ID,
		CustomerId:     n.CustomerId,
		NoteText:       n.NoteText,
		NextAction:     n.NextAction,
		NextActionDate: n.NextActionDate,
		CreatedBy:      n.CreatedBy,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}

func getUserIDFromContext(c fiber.Ctx) *primitive.ObjectID {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok || userIDStr == "" {
		return nil
	}
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return nil
	}
	return &userID
}
