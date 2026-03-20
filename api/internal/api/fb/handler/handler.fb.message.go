package fbhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"

	"github.com/gofiber/fiber/v3"
)

// FbMessageHandler xử lý các yêu cầu liên quan đến Facebook Message
type FbMessageHandler struct {
	*basehdl.BaseHandler[fbmodels.FbMessage, fbdto.FbMessageCreateInput, fbdto.FbMessageCreateInput]
	FbMessageService *fbsvc.FbMessageService
}

// NewFbMessageHandler khởi tạo FbMessageHandler mới
func NewFbMessageHandler() (*FbMessageHandler, error) {
	service, err := fbsvc.NewFbMessageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create message service: %v", err)
	}
	hdl := &FbMessageHandler{
		BaseHandler:      basehdl.NewBaseHandler[fbmodels.FbMessage, fbdto.FbMessageCreateInput, fbdto.FbMessageCreateInput](service.BaseServiceMongoImpl),
		FbMessageService: service,
	}
	return hdl, nil
}

// UpsertMessagesFromParts body JSON — logic ở FbMessageService.RunUpsertMessagesFromJSON (CIO ingest domain interaction_message).
func (h *FbMessageHandler) UpsertMessagesFromParts(c fiber.Ctx, body []byte) error {
	result, err := h.FbMessageService.RunUpsertMessagesFromJSON(c.Context(), body)
	h.HandleResponse(c, result, err)
	return nil
}
