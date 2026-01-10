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

// DraftVideoHandler xử lý các request liên quan đến Draft Video (L7)
type DraftVideoHandler struct {
	BaseHandler[models.DraftVideo, dto.DraftVideoCreateInput, dto.DraftVideoUpdateInput]
	DraftVideoService *services.DraftVideoService
}

// NewDraftVideoHandler tạo mới DraftVideoHandler
func NewDraftVideoHandler() (*DraftVideoHandler, error) {
	draftVideoService, err := services.NewDraftVideoService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft video service: %v", err)
	}

	handler := &DraftVideoHandler{
		DraftVideoService: draftVideoService,
	}
	handler.BaseService = handler.DraftVideoService.BaseServiceMongoImpl

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
func (h *DraftVideoHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.DraftVideoCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate DraftScriptID
		if !primitive.IsValidObjectID(input.DraftScriptID) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("DraftScriptID '%s' không đúng định dạng MongoDB ObjectID", input.DraftScriptID),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		draftVideo := models.DraftVideo{
			DraftScriptID: utility.String2ObjectID(input.DraftScriptID),
			AssetURL:       input.AssetURL,
			ThumbnailURL:   input.ThumbnailURL,
			Meta:           input.Meta,
		}

		// Set status (mặc định: pending)
		if input.Status == "" {
			draftVideo.Status = models.VideoStatusPending
		} else {
			draftVideo.Status = input.Status
		}

		// Set approval status (mặc định: draft)
		if input.ApprovalStatus == "" {
			draftVideo.ApprovalStatus = models.DraftApprovalStatusDraft
		} else {
			draftVideo.ApprovalStatus = input.ApprovalStatus
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), draftVideo)
		h.HandleResponse(c, data, err)
		return nil
	})
}
