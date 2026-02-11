package fbhdl

import (
	"context"
	"fmt"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	basehdl "meta_commerce/internal/api/base/handler"

	"github.com/gofiber/fiber/v3"
)

// FbPostHandler xử lý các yêu cầu liên quan đến Facebook Post
type FbPostHandler struct {
	*basehdl.BaseHandler[fbmodels.FbPost, fbdto.FbPostCreateInput, fbdto.FbPostCreateInput]
	FbPostService *fbsvc.FbPostService
}

// NewFbPostHandler khởi tạo FbPostHandler mới
func NewFbPostHandler() (*FbPostHandler, error) {
	service, err := fbsvc.NewFbPostService()
	if err != nil {
		return nil, fmt.Errorf("failed to create post service: %v", err)
	}
	hdl := &FbPostHandler{FbPostService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[fbmodels.FbPost, fbdto.FbPostCreateInput, fbdto.FbPostCreateInput](service)
	return hdl, nil
}

// HandleFindOneByPostID tìm một FbPost theo PostID
func (h *FbPostHandler) HandleFindOneByPostID(c fiber.Ctx) error {
	id := h.GetIDFromContext(c)
	data, err := h.FbPostService.FindOneByPostID(context.Background(), id)
	h.HandleResponse(c, data, err)
	return nil
}
