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

// PublicationHandler xử lý các request liên quan đến Publication (L8)
type PublicationHandler struct {
	BaseHandler[models.Publication, dto.PublicationCreateInput, dto.PublicationUpdateInput]
	PublicationService *services.PublicationService
}

// NewPublicationHandler tạo mới PublicationHandler
func NewPublicationHandler() (*PublicationHandler, error) {
	publicationService, err := services.NewPublicationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create publication service: %v", err)
	}

	handler := &PublicationHandler{
		PublicationService: publicationService,
	}
	handler.BaseService = handler.PublicationService.BaseServiceMongoImpl

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
func (h *PublicationHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.PublicationCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate VideoID
		if !primitive.IsValidObjectID(input.VideoID) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("VideoID '%s' không đúng định dạng MongoDB ObjectID", input.VideoID),
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
		publication := models.Publication{
			VideoID:        utility.String2ObjectID(input.VideoID),
			Platform:       input.Platform,
			PlatformPostID: input.PlatformPostID,
			MetricsRaw:     input.MetricsRaw,
			Metadata:       input.Metadata,
			ScheduledAt:    input.ScheduledAt,
			PublishedAt:    input.PublishedAt,
		}

		// Set status (mặc định: draft)
		if input.Status == "" {
			publication.Status = models.PublicationStatusDraft
		} else {
			publication.Status = input.Status
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), publication)
		h.HandleResponse(c, data, err)
		return nil
	})
}
