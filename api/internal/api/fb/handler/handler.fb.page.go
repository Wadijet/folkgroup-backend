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

// FbPageHandler xử lý các yêu cầu liên quan đến Facebook Page
type FbPageHandler struct {
	*basehdl.BaseHandler[fbmodels.FbPage, fbdto.FbPageCreateInput, fbdto.FbPageCreateInput]
	FbPageService *fbsvc.FbPageService
}

// NewFbPageHandler khởi tạo FbPageHandler mới
func NewFbPageHandler() (*FbPageHandler, error) {
	service, err := fbsvc.NewFbPageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create page service: %v", err)
	}
	hdl := &FbPageHandler{FbPageService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[fbmodels.FbPage, fbdto.FbPageCreateInput, fbdto.FbPageCreateInput](service)
	return hdl, nil
}

// HandleFindOneByPageID tìm một FbPage theo PageID
func (h *FbPageHandler) HandleFindOneByPageID(c fiber.Ctx) error {
	id := h.GetIDFromContext(c)
	data, err := h.FbPageService.FindOneByPageID(context.Background(), id)
	h.HandleResponse(c, data, err)
	return nil
}

// HandleUpdateToken cập nhật access token của một FbPage
func (h *FbPageHandler) HandleUpdateToken(c fiber.Ctx) error {
	input := new(fbdto.FbPageUpdateTokenInput)
	if err := h.ParseRequestBody(c, input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	data, err := h.FbPageService.UpdateToken(context.Background(), input)
	h.HandleResponse(c, data, err)
	return nil
}
