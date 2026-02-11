// Package ctahdl chứa HTTP handler cho domain CTA (Library).
// File: basehdl.cta.library.go - giữ tên cấu trúc cũ (basehdl.<domain>.<entity>.go).
package ctahdl

import (
	"fmt"

	ctadto "meta_commerce/internal/api/cta/dto"
	ctamodels "meta_commerce/internal/api/cta/models"
	ctasvc "meta_commerce/internal/api/cta/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// CTALibraryHandler xử lý các request liên quan đến CTA Library
type CTALibraryHandler struct {
	basehdl.BaseHandler[ctamodels.CTALibrary, ctadto.CTALibraryCreateInput, ctadto.CTALibraryUpdateInput]
}

// NewCTALibraryHandler tạo mới CTALibraryHandler
func NewCTALibraryHandler() (*CTALibraryHandler, error) {
	ctaLibraryService, err := ctasvc.NewCTALibraryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create CTA library service: %v", err)
	}

	baseHandler := basehdl.NewBaseHandler[ctamodels.CTALibrary, ctadto.CTALibraryCreateInput, ctadto.CTALibraryUpdateInput](ctaLibraryService)
	h := &CTALibraryHandler{
		BaseHandler: *baseHandler,
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields: []string{},
		AllowedOperators: []string{
			"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists",
		},
		MaxFields: 10,
	})
	return h, nil
}
