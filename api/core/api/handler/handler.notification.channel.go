package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationChannelHandler xử lý các request liên quan đến Notification Channel
type NotificationChannelHandler struct {
	BaseHandler[models.NotificationChannel, dto.NotificationChannelCreateInput, dto.NotificationChannelUpdateInput]
	channelService *services.NotificationChannelService
}

// NewNotificationChannelHandler tạo mới NotificationChannelHandler
func NewNotificationChannelHandler() (*NotificationChannelHandler, error) {
	channelService, err := services.NewNotificationChannelService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification channel service: %v", err)
	}

	baseHandler := NewBaseHandler[models.NotificationChannel, dto.NotificationChannelCreateInput, dto.NotificationChannelUpdateInput](channelService)
	handler := &NotificationChannelHandler{
		BaseHandler:    *baseHandler,
		channelService: channelService,
	}

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{},
		AllowedOperators: []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		},
		MaxFields: 10,
	}

	return handler, nil
}

// InsertOne override để thêm validation uniqueness
// Mỗi organization chỉ có thể có 1 channel với cùng tên và channelType
// Và không được có duplicate recipients (email/telegram) hoặc webhookUrl (webhook)
func (h *NotificationChannelHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành struct T
		input := new(models.NotificationChannel)
		if err := h.ParseRequestBody(c, input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(input)
		if ownerOrgIDFromRequest != nil && !ownerOrgIDFromRequest.IsZero() {
			// Có ownerOrganizationId trong request → Validate quyền
			if err := h.validateUserHasAccessToOrg(c, *ownerOrgIDFromRequest); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
			// ✅ Có quyền → Giữ nguyên ownerOrganizationId từ request
		} else {
			// Không có trong request → Dùng context (backward compatible)
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				h.setOrganizationID(input, *activeOrgID)
			} else {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationInput,
					"Không thể xác định organization. Vui lòng cung cấp ownerOrganizationId hoặc set active organization context",
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
		}

		// ✅ Validate uniqueness: Kiểm tra đã có channel với cùng name, channelType và ownerOrganizationId chưa
		ownerOrgID := h.getOwnerOrganizationIDFromModel(input)
		if ownerOrgID != nil && !ownerOrgID.IsZero() && input.Name != "" && input.ChannelType != "" {
			filter := bson.M{
				"ownerOrganizationId": *ownerOrgID,
				"channelType":         input.ChannelType,
				"name":                input.Name,
				// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
			}
			_, err := h.channelService.FindOne(c.Context(), filter, nil)
			if err == nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("Đã tồn tại channel với tên '%s' và channelType '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel với cùng tên và channelType", input.Name, input.ChannelType),
					common.StatusConflict,
					nil,
				))
				return nil
			}
			if err != common.ErrNotFound {
				h.HandleResponse(c, nil, err)
				return nil
			}
		}

		// ✅ Validate duplicate recipients/webhookUrl
		if ownerOrgID != nil && !ownerOrgID.IsZero() {
			// Check duplicate recipients cho email
			if input.ChannelType == "email" && len(input.Recipients) > 0 {
				for _, recipient := range input.Recipients {
					// Check trong array recipients (MongoDB $in operator)
					filter := bson.M{
						"ownerOrganizationId": *ownerOrgID,
						"channelType":         "email",
						"recipients":          bson.M{"$in": []string{recipient}},
						// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
					}
					existing, err := h.channelService.FindOne(c.Context(), filter, nil)
					if err == nil {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeBusinessOperation,
							fmt.Sprintf("Đã tồn tại email channel với recipient '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi recipient", recipient),
							common.StatusConflict,
							nil,
						))
						return nil
					}
					if err != common.ErrNotFound {
						h.HandleResponse(c, nil, err)
						return nil
					}
					_ = existing // Tránh unused variable warning
				}
			}

			// Check duplicate chatIDs cho telegram
			if input.ChannelType == "telegram" && len(input.ChatIDs) > 0 {
				for _, chatID := range input.ChatIDs {
					// Check trong array chatIds (MongoDB $in operator)
					filter := bson.M{
						"ownerOrganizationId": *ownerOrgID,
						"channelType":         "telegram",
						"chatIds":             bson.M{"$in": []string{chatID}},
						// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
					}
					existing, err := h.channelService.FindOne(c.Context(), filter, nil)
					if err == nil {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeBusinessOperation,
							fmt.Sprintf("Đã tồn tại telegram channel với chatID '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi chatID", chatID),
							common.StatusConflict,
							nil,
						))
						return nil
					}
					if err != common.ErrNotFound {
						h.HandleResponse(c, nil, err)
						return nil
					}
					_ = existing // Tránh unused variable warning
				}
			}

			// Check duplicate webhookUrl cho webhook
			if input.ChannelType == "webhook" && input.WebhookURL != "" {
				filter := bson.M{
					"ownerOrganizationId": *ownerOrgID,
					"channelType":         "webhook",
					"webhookUrl":          input.WebhookURL,
					// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
				}
				_, err := h.channelService.FindOne(c.Context(), filter, nil)
				if err == nil {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("Đã tồn tại webhook channel với URL '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi webhook URL", input.WebhookURL),
						common.StatusConflict,
						nil,
					))
					return nil
				}
				if err != common.ErrNotFound {
					h.HandleResponse(c, nil, err)
					return nil
				}
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		data, err := h.BaseService.InsertOne(ctx, *input)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		h.HandleResponse(c, data, nil)
		return nil
	})
}
