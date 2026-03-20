package fbhdl

import (
	"context"
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	fbdto "meta_commerce/internal/api/fb/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	fbsvc "meta_commerce/internal/api/fb/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
)

// FbConversationHandler xử lý các route liên quan đến Facebook Conversation
type FbConversationHandler struct {
	*basehdl.BaseHandler[fbmodels.FbConversation, fbdto.FbConversationCreateInput, fbdto.FbConversationCreateInput]
	FbConversationService *fbsvc.FbConversationService
}

// NewFbConversationHandler tạo FbConversationHandler mới
func NewFbConversationHandler() (*FbConversationHandler, error) {
	service, err := fbsvc.NewFbConversationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation service: %v", err)
	}
	hdl := &FbConversationHandler{
		FbConversationService: service,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[fbmodels.FbConversation, fbdto.FbConversationCreateInput, fbdto.FbConversationCreateInput](service.BaseServiceMongoImpl)
	return hdl, nil
}

// HandleFindAllSortByApiUpdate tìm tất cả FbConversation với phân trang sắp xếp theo thời gian cập nhật API
func (h *FbConversationHandler) HandleFindAllSortByApiUpdate(c fiber.Ctx) error {
	pageInt, limitInt := h.ParsePagination(c)
	page := int64(pageInt)
	limit := int64(limitInt)

	filter := bson.M{}
	if pageId := c.Query("pageId"); pageId != "" {
		filter = bson.M{"pageId": pageId}
	}

	result, err := h.FbConversationService.FindAllSortByApiUpdate(context.Background(), page, limit, filter)
	h.HandleResponse(c, result, err)
	return nil
}

// SyncUpsertOneFromParts filter + body — logic ở FbConversationService.RunSyncUpsertOneFromJSON.
func (h *FbConversationHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.FbConversationService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}
