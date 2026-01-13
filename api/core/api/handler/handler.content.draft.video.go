package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
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

