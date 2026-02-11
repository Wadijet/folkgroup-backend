package contenthdl

import (
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// DraftPublicationHandler xử lý các request liên quan đến Draft Publication (L8)
type DraftPublicationHandler struct {
	*basehdl.BaseHandler[contentmodels.DraftPublication, contentdto.DraftPublicationCreateInput, contentdto.DraftPublicationUpdateInput]
	DraftPublicationService *contentsvc.DraftPublicationService
}

// NewDraftPublicationHandler tạo mới DraftPublicationHandler
func NewDraftPublicationHandler() (*DraftPublicationHandler, error) {
	draftPublicationService, err := contentsvc.NewDraftPublicationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft publication service: %v", err)
	}
	hdl := &DraftPublicationHandler{DraftPublicationService: draftPublicationService}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.DraftPublication, contentdto.DraftPublicationCreateInput, contentdto.DraftPublicationUpdateInput](draftPublicationService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
