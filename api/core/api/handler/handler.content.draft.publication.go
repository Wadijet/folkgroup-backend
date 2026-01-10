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

// DraftPublicationHandler xử lý các request liên quan đến Draft Publication (L8)
type DraftPublicationHandler struct {
	BaseHandler[models.DraftPublication, dto.DraftPublicationCreateInput, dto.DraftPublicationUpdateInput]
	DraftPublicationService *services.DraftPublicationService
}

// NewDraftPublicationHandler tạo mới DraftPublicationHandler
func NewDraftPublicationHandler() (*DraftPublicationHandler, error) {
	draftPublicationService, err := services.NewDraftPublicationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft publication service: %v", err)
	}

	handler := &DraftPublicationHandler{
		DraftPublicationService: draftPublicationService,
	}
	handler.BaseService = handler.DraftPublicationService.BaseServiceMongoImpl

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
func (h *DraftPublicationHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.DraftPublicationCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate DraftVideoID
		if !primitive.IsValidObjectID(input.DraftVideoID) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("DraftVideoID '%s' không đúng định dạng MongoDB ObjectID", input.DraftVideoID),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Validate Platform
		validPlatforms := []string{
			models.PublicationPlatformFacebook,
			models.PublicationPlatformTikTok,
			models.PublicationPlatformYouTube,
			models.PublicationPlatformInstagram,
		}
		platformValid := false
		for _, validPlatform := range validPlatforms {
			if input.Platform == validPlatform {
				platformValid = true
				break
			}
		}
		if !platformValid {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Platform '%s' không hợp lệ. Các giá trị hợp lệ: %v", input.Platform, validPlatforms),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		draftPublication := models.DraftPublication{
			DraftVideoID:   utility.String2ObjectID(input.DraftVideoID),
			Platform:       input.Platform,
			PlatformPostID: input.PlatformPostID,
			Metadata:       input.Metadata,
			ScheduledAt:    input.ScheduledAt,
		}

		// Set status (mặc định: draft)
		if input.Status == "" {
			draftPublication.Status = models.PublicationStatusDraft
		} else {
			draftPublication.Status = input.Status
		}

		// Set approval status (mặc định: draft)
		if input.ApprovalStatus == "" {
			draftPublication.ApprovalStatus = models.DraftApprovalStatusDraft
		} else {
			draftPublication.ApprovalStatus = input.ApprovalStatus
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), draftPublication)
		h.HandleResponse(c, data, err)
		return nil
	})
}
