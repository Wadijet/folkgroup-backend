package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// CTALibraryHandler xử lý các request liên quan đến CTA Library
type CTALibraryHandler struct {
	BaseHandler[models.CTALibrary, dto.CTALibraryCreateInput, dto.CTALibraryUpdateInput]
}

// NewCTALibraryHandler tạo mới CTALibraryHandler
func NewCTALibraryHandler() (*CTALibraryHandler, error) {
	ctaLibraryService, err := services.NewCTALibraryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create CTA library service: %v", err)
	}

	baseHandler := NewBaseHandler[models.CTALibrary, dto.CTALibraryCreateInput, dto.CTALibraryUpdateInput](ctaLibraryService)
	handler := &CTALibraryHandler{
		BaseHandler: *baseHandler,
	}

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{},
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
