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

// VideoHandler xử lý các request liên quan đến Video (L7)
type VideoHandler struct {
	BaseHandler[models.Video, dto.VideoCreateInput, dto.VideoUpdateInput]
	VideoService *services.VideoService
}

// NewVideoHandler tạo mới VideoHandler
func NewVideoHandler() (*VideoHandler, error) {
	videoService, err := services.NewVideoService()
	if err != nil {
		return nil, fmt.Errorf("failed to create video service: %v", err)
	}

	handler := &VideoHandler{
		VideoService: videoService,
	}
	handler.BaseService = handler.VideoService.BaseServiceMongoImpl

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
func (h *VideoHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.VideoCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate ScriptID
		if !primitive.IsValidObjectID(input.ScriptID) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ScriptID '%s' không đúng định dạng MongoDB ObjectID", input.ScriptID),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		video := models.Video{
			ScriptID:     utility.String2ObjectID(input.ScriptID),
			AssetURL:     input.AssetURL,
			ThumbnailURL: input.ThumbnailURL,
			Meta:         input.Meta,
		}

		// Set status (mặc định: pending)
		if input.Status == "" {
			video.Status = models.VideoStatusPending
		} else {
			video.Status = input.Status
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), video)
		h.HandleResponse(c, data, err)
		return nil
	})
}
