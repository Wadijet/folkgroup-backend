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

// NotificationRoutingHandler xử lý các request liên quan đến Notification Routing Rule
type NotificationRoutingHandler struct {
	BaseHandler[models.NotificationRoutingRule, dto.NotificationRoutingRuleCreateInput, dto.NotificationRoutingRuleUpdateInput]
	routingService *services.NotificationRoutingService
}

// NewNotificationRoutingHandler tạo mới NotificationRoutingHandler
func NewNotificationRoutingHandler() (*NotificationRoutingHandler, error) {
	routingService, err := services.NewNotificationRoutingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification routing service: %v", err)
	}

	baseHandler := NewBaseHandler[models.NotificationRoutingRule, dto.NotificationRoutingRuleCreateInput, dto.NotificationRoutingRuleUpdateInput](routingService)
	handler := &NotificationRoutingHandler{
		BaseHandler:    *baseHandler,
		routingService: routingService,
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
//
// LÝ DO PHẢI OVERRIDE (không thể dùng CRUD chuẩn):
// 1. Validation uniqueness phức tạp:
//    - Mỗi organization chỉ có thể có 1 rule cho mỗi eventType (eventType + ownerOrganizationId là unique)
//    - Mỗi organization chỉ có thể có 1 rule cho mỗi domain (domain + ownerOrganizationId là unique)
//    - Chỉ check rules đang active (isActive = true)
//    - Cần query database để check duplicate trước khi insert
// 2. Validation nghiệp vụ:
//    - EventType là bắt buộc và không được để trống
//    - Nếu có Domain, cũng phải validate uniqueness cho Domain
// 3. Logic đặc biệt:
//    - Parse trực tiếp vào Model (không dùng DTO) vì cần validate uniqueness dựa trên Model fields
//    - Validate quyền với ownerOrganizationId (nếu có trong request)
//    - Set ownerOrganizationId từ context nếu không có trong request
//
// KẾT LUẬN: Cần giữ override vì validation uniqueness phức tạp (query database để check duplicate)
//           và logic nghiệp vụ đặc biệt (chỉ check rules active, validate eventType bắt buộc)
func (h *NotificationRoutingHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành struct T
		input := new(models.NotificationRoutingRule)
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

		// ✅ Validate EventType: EventType là bắt buộc
		if input.EventType == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationInput,
				"EventType là bắt buộc và không được để trống",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Validate uniqueness: Kiểm tra đã có rule cho eventType và ownerOrganizationId chưa
		ownerOrgID := h.getOwnerOrganizationIDFromModel(input)
		if ownerOrgID != nil && !ownerOrgID.IsZero() {
			// Kiểm tra rule với eventType (EventType là bắt buộc)
			filter := bson.M{
				"eventType":           input.EventType, // EventType giờ là string, không phải *string
				"ownerOrganizationId": *ownerOrgID,
				"isActive":            true, // Chỉ check rules đang active
			}
			_, err := h.routingService.FindOne(c.Context(), filter, nil)
			if err == nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("Đã tồn tại routing rule cho eventType '%s' và organization này. Mỗi organization chỉ có thể có 1 rule cho mỗi eventType", input.EventType),
					common.StatusConflict,
					nil,
				))
				return nil
			}
			if err != common.ErrNotFound {
				h.HandleResponse(c, nil, err)
				return nil
			}

			// Kiểm tra rule với domain (nếu có)
			if input.Domain != nil && *input.Domain != "" {
				filter := bson.M{
					"domain":              *input.Domain,
					"ownerOrganizationId": *ownerOrgID,
					"isActive":            true, // Chỉ check rules đang active
				}
				_, err := h.routingService.FindOne(c.Context(), filter, nil)
				if err == nil {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("Đã tồn tại routing rule cho domain '%s' và organization này. Mỗi organization chỉ có thể có 1 rule cho mỗi domain", *input.Domain),
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
		h.HandleResponse(c, data, err)
		return nil
	})
}
