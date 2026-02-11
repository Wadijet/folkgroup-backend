package contenthdl

import (
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// DraftVideoHandler xử lý các request liên quan đến Draft Video (L7)
type DraftVideoHandler struct {
	*basehdl.BaseHandler[contentmodels.DraftVideo, contentdto.DraftVideoCreateInput, contentdto.DraftVideoUpdateInput]
	DraftVideoService *contentsvc.DraftVideoService
}

// NewDraftVideoHandler tạo mới DraftVideoHandler
func NewDraftVideoHandler() (*DraftVideoHandler, error) {
	draftVideoService, err := contentsvc.NewDraftVideoService()
	if err != nil {
		return nil, fmt.Errorf("failed to create draft video service: %v", err)
	}
	hdl := &DraftVideoHandler{DraftVideoService: draftVideoService}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.DraftVideo, contentdto.DraftVideoCreateInput, contentdto.DraftVideoUpdateInput](draftVideoService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
