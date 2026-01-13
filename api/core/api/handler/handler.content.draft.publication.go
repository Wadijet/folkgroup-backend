package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
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

