package contenthdl

import (
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// PublicationHandler xử lý các request liên quan đến Publication (L8)
type PublicationHandler struct {
	*basehdl.BaseHandler[contentmodels.Publication, contentdto.PublicationCreateInput, contentdto.PublicationUpdateInput]
	PublicationService *contentsvc.PublicationService
}

// NewPublicationHandler tạo mới PublicationHandler
func NewPublicationHandler() (*PublicationHandler, error) {
	publicationService, err := contentsvc.NewPublicationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create publication service: %v", err)
	}
	hdl := &PublicationHandler{PublicationService: publicationService}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.Publication, contentdto.PublicationCreateInput, contentdto.PublicationUpdateInput](publicationService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
