package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
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

