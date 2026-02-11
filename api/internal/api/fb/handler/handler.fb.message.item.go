package fbhdl

import (
	"context"
	"fmt"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// FbMessageItemHandler xử lý các yêu cầu liên quan đến Facebook Message Item
type FbMessageItemHandler struct {
	*basehdl.BaseHandler[fbmodels.FbMessageItem, fbdto.FbMessageItemCreateInput, fbdto.FbMessageItemUpdateInput]
	FbMessageItemService *fbsvc.FbMessageItemService
}

// NewFbMessageItemHandler khởi tạo FbMessageItemHandler mới
func NewFbMessageItemHandler() (*FbMessageItemHandler, error) {
	service, err := fbsvc.NewFbMessageItemService()
	if err != nil {
		return nil, fmt.Errorf("failed to create message item service: %v", err)
	}
	hdl := &FbMessageItemHandler{
		BaseHandler:          basehdl.NewBaseHandler[fbmodels.FbMessageItem, fbdto.FbMessageItemCreateInput, fbdto.FbMessageItemUpdateInput](service.BaseServiceMongoImpl),
		FbMessageItemService: service,
	}
	return hdl, nil
}

// HandleFindByConversationId tìm tất cả message items của một conversation với phân trang
func (h *FbMessageItemHandler) HandleFindByConversationId(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		conversationId := c.Params("conversationId")
		if conversationId == "" {
			h.HandleResponse(c, nil, fmt.Errorf("conversationId không được để trống"))
			return nil
		}

		page, err := strconv.ParseInt(c.Query("page", "1"), 10, 64)
		if err != nil || page < 1 {
			page = 1
		}
		limit, err := strconv.ParseInt(c.Query("limit", "50"), 10, 64)
		if err != nil || limit < 1 || limit > 100 {
			limit = 50
		}

		messages, total, err := h.FbMessageItemService.FindByConversationId(
			context.Background(),
			conversationId,
			int64(page),
			int64(limit),
		)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		result := map[string]interface{}{
			"data": messages,
			"pagination": map[string]interface{}{
				"page":  page,
				"limit": limit,
				"total": total,
			},
		}
		h.HandleResponse(c, result, nil)
		return nil
	})
}

// HandleFindOneByMessageId tìm một FbMessageItem theo MessageId
func (h *FbMessageItemHandler) HandleFindOneByMessageId(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		messageId := c.Params("messageId")
		if messageId == "" {
			h.HandleResponse(c, nil, fmt.Errorf("messageId không được để trống"))
			return nil
		}
		filter := map[string]interface{}{"messageId": messageId}
		data, err := h.FbMessageItemService.FindOne(context.Background(), filter, nil)
		h.HandleResponse(c, data, err)
		return nil
	})
}
