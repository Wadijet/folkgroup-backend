package fbhdl

import (
	"fmt"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

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

// HandleUpsertMessages xử lý upsert messages từ Pancake API
func (h *FbMessageHandler) HandleUpsertMessages(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var input fbdto.FbMessageUpsertMessagesInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		if err := global.Validate.Struct(input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu không hợp lệ: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		result, err := h.FbMessageService.UpsertMessages(
			c.Context(),
			input.ConversationId,
			input.PageId,
			input.PageUsername,
			input.CustomerId,
			input.PanCakeData,
			input.HasMore,
		)

		h.HandleResponse(c, result, err)
		return nil
	})
}
