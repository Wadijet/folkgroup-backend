package contenthdl

import (
	"fmt"
	contentdto "meta_commerce/internal/api/content/dto"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// VideoHandler xử lý các request liên quan đến Video (L7)
type VideoHandler struct {
	*basehdl.BaseHandler[contentmodels.Video, contentdto.VideoCreateInput, contentdto.VideoUpdateInput]
	VideoService *contentsvc.VideoService
}

// NewVideoHandler tạo mới VideoHandler
func NewVideoHandler() (*VideoHandler, error) {
	videoService, err := contentsvc.NewVideoService()
	if err != nil {
		return nil, fmt.Errorf("failed to create video service: %v", err)
	}
	hdl := &VideoHandler{VideoService: videoService}
	hdl.BaseHandler = basehdl.NewBaseHandler[contentmodels.Video, contentdto.VideoCreateInput, contentdto.VideoUpdateInput](videoService.BaseServiceMongoImpl)
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{"password", "token", "secret", "key", "hash"},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
