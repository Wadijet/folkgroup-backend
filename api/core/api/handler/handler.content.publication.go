package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
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

