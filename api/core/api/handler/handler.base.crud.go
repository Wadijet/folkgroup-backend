package handler

// Package handler chứa các handler xử lý request HTTP trong ứng dụng.
// Package này cung cấp các chức năng CRUD cơ bản và các tiện ích để xử lý request/response.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"
	"reflect"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// InsertOne thêm mới một document vào database.
// Dữ liệu được parse từ request body (DTO CreateInput) và transform sang Model trước khi thêm vào DB.
// Sử dụng struct tag `transform` trong DTO để tự động convert các field (ví dụ: string → ObjectID).
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO (CreateInput)
		var input CreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Validate input với struct tag (validate, oneof, etc.)
		if err := h.validateInput(&input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Transform DTO sang Model sử dụng struct tag `transform`
		model, err := h.transformCreateInputToModel(&input)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi transform dữ liệu: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(model)
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
				h.setOrganizationID(model, *activeOrgID)
			}
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		data, err := h.BaseService.InsertOne(ctx, *model)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// InsertMany thêm nhiều document vào database.
// Dữ liệu được parse từ request body dưới dạng mảng và validate trước khi thêm vào DB.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertMany(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var inputs []T
		if err := h.ParseRequestBody(c, &inputs); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên phải là một mảng JSON và các phần tử phải khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId cho tất cả items: Cho phép chỉ định từ request hoặc dùng context
		for i := range inputs {
			ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(&inputs[i])
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
					h.setOrganizationID(&inputs[i], *activeOrgID)
				}
			}
		}

		data, err := h.BaseService.InsertMany(c.Context(), inputs)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// FindOne tìm một document theo điều kiện filter.
// Filter và options được truyền qua query string dưới dạng JSON.
// Ví dụ options: {"projection": {"field": 1}, "sort": {"field": 1}}
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		options, err := h.processMongoOptions(c, true)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		data, err := h.BaseService.FindOne(c.Context(), filter, options.(*mongoopts.FindOneOptions))
		h.HandleResponse(c, data, err)
		return nil
	})
}

// FindOneById tìm một document theo ID.
// ID được truyền qua URI params.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOneById(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		id := c.Params("id")
		if id == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"ID không được để trống trong URL params",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		if !primitive.IsValidObjectID(id) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ID '%s' không đúng định dạng MongoDB ObjectID (phải là chuỗi hex 24 ký tự)", id),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Validate ownerOrganizationId trước khi query nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		data, err := h.BaseService.FindOneById(c.Context(), utility.String2ObjectID(id))
		h.HandleResponse(c, data, err)
		return nil
	})
}

// FindManyByIds tìm nhiều document theo danh sách ID.
// Danh sách ID được truyền qua query string dưới dạng mảng JSON.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindManyByIds(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var ids []string
		idsStr := c.Query("ids", "[]")
		if err := json.Unmarshal([]byte(idsStr), &ids); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Danh sách ID phải là một mảng JSON. Giá trị nhận được: %s. Chi tiết lỗi: %v", idsStr, err),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Validate từng ID
		objectIds := make([]primitive.ObjectID, len(ids))
		for i, id := range ids {
			if !primitive.IsValidObjectID(id) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("ID '%s' tại vị trí %d không đúng định dạng MongoDB ObjectID (phải là chuỗi hex 24 ký tự)", id, i),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			objectIds[i] = utility.String2ObjectID(id)
		}

		data, err := h.BaseService.FindManyByIds(c.Context(), objectIds)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// FindWithPagination tìm nhiều document với phân trang.
// Hỗ trợ filter, options và phân trang với page và limit.
//
// Parameters:
// - c: Fiber context
// Query params:
// - filter: Điều kiện tìm kiếm (JSON)
// - options: Tùy chọn tìm kiếm (JSON). Ví dụ: {"projection": {"field": 1}, "sort": {"field": 1}}
// - page: Số trang (mặc định: 1)
// - limit: Số lượng item trên một trang (mặc định: 10)
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindWithPagination(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Sử dụng processFilter để có normalizeFilter và validate
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		options, err := h.processMongoOptions(c, false)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Parse page và limit từ query string
		page, err := strconv.ParseInt(c.Query("page", "1"), 10, 64)
		if err != nil {
			page = 1
		}
		// Đảm bảo page >= 1 để tránh skip âm
		if page < 1 {
			page = 1
		}

		limit, err := strconv.ParseInt(c.Query("limit", "10"), 10, 64)
		if err != nil {
			limit = 10
		}
		// Đảm bảo limit > 0
		if limit <= 0 {
			limit = 10
		}

		// Không set limit và skip vào options ở đây
		// Service sẽ tự tính toán và set vào options để đảm bảo tính nhất quán
		findOptions := options.(*mongoopts.FindOptions)

		data, err := h.BaseService.FindWithPagination(c.Context(), filter, page, limit, findOptions)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// Find tìm nhiều document theo điều kiện filter.
// Filter và options được truyền qua query string dưới dạng JSON.
// Ví dụ options: {"projection": {"field": 1}, "sort": {"field": 1}, "limit": 10, "skip": 0}
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) Find(c fiber.Ctx) error {
	// DEBUG: Log khi handler được gọi
	fmt.Printf("[HANDLER] 🔵 Find handler called - Path: %s, Method: %s\n", c.Path(), c.Method())
	logrus.WithFields(logrus.Fields{
		"path":   c.Path(),
		"method": c.Method(),
	}).Info("🔵 Find handler called")

	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		options, err := h.processMongoOptions(c, false)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		data, err := h.BaseService.Find(c.Context(), filter, options.(*mongoopts.FindOptions))
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Đảm bảo data không bao giờ là nil, luôn trả về mảng rỗng nếu không có kết quả
		if data == nil {
			data = []T{}
		}

		h.HandleResponse(c, data, nil)
		return nil
	})
}

// UpdateOne cập nhật một document theo điều kiện filter.
// Filter được truyền qua query string, dữ liệu cập nhật trong request body.
// Chỉ update các trường có trong input, giữ nguyên các trường khác.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpdateOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		// Parse request body thành DTO (UpdateInput)
		var input UpdateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Validate input với struct tag (validate, oneof, etc.)
		if err := h.validateInput(&input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Transform DTO sang Model sử dụng struct tag `transform` (hỗ trợ nested struct)
		model, err := h.transformUpdateInputToModel(&input)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi transform dữ liệu: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép update với validation quyền
		// Lưu ý: UpdateOne không có document ID riêng, cần validate qua filter
		// Nếu có ownerOrganizationId trong model, validate quyền với organization mới
		ownerOrgIDFromModel := h.getOwnerOrganizationIDFromModel(model)
		if ownerOrgIDFromModel != nil && !ownerOrgIDFromModel.IsZero() {
			// Validate user có quyền với organization mới
			if err := h.validateUserHasAccessToOrg(c, *ownerOrgIDFromModel); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
		}

		// Convert model sang UpdateData với $set operator
		updateData := &services.UpdateData{
			Set: make(map[string]interface{}),
		}
		// Convert model sang map để set vào $set
		modelBytes, err := bson.Marshal(model)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi convert model sang BSON: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}
		var modelMap map[string]interface{}
		if err := bson.Unmarshal(modelBytes, &modelMap); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi unmarshal BSON: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}
		// Set các field vào $set (loại bỏ zero values)
		for k, v := range modelMap {
			if !reflect.ValueOf(v).IsZero() {
				updateData.Set[k] = v
			}
		}

		// Tạo update data với $set operator
		update := updateData

		data, err := h.BaseService.UpdateOne(c.Context(), filter, update, nil)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// UpdateMany cập nhật nhiều document theo điều kiện filter.
// Filter được truyền qua query string, dữ liệu cập nhật trong request body.
// Chỉ update các trường có trong input, giữ nguyên các trường khác.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpdateMany(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		// Parse input thành map để chỉ update các trường được chỉ định
		var updateData map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&updateData); err != nil {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu cập nhật không hợp lệ", common.StatusBadRequest, nil))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép update với validation quyền
		// Lưu ý: UpdateMany không có document ID riêng, cần validate qua filter
		// Nếu có ownerOrganizationId trong updateData, validate quyền với organization mới
		if newOwnerOrgIDStr, ok := updateData["ownerOrganizationId"].(string); ok && newOwnerOrgIDStr != "" {
			// Parse ObjectID
			newOwnerOrgID, err := primitive.ObjectIDFromHex(newOwnerOrgIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"ownerOrganizationId không hợp lệ",
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			// Validate user có quyền với organization mới
			if err := h.validateUserHasAccessToOrg(c, newOwnerOrgID); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}

			// ✅ Có quyền → Thay thế string bằng ObjectID trong updateData để MongoDB lưu đúng kiểu
			updateData["ownerOrganizationId"] = newOwnerOrgID
		} else {
			// Không có ownerOrganizationId trong update → Xóa nếu có (giữ nguyên logic cũ)
			delete(updateData, "ownerOrganizationId")
		}

		// Tạo update data với $set operator
		update := &services.UpdateData{
			Set: updateData,
		}

		count, err := h.BaseService.UpdateMany(c.Context(), filter, update, nil)
		h.HandleResponse(c, count, err)
		return nil
	})
}

// UpdateById cập nhật một document theo ID.
// ID được truyền qua URI params, dữ liệu cập nhật trong request body.
// Chỉ update các trường có trong input, giữ nguyên các trường khác.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpdateById(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		id := c.Params("id")
		if id == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"ID không được để trống trong URL params",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		if !primitive.IsValidObjectID(id) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ID '%s' không đúng định dạng MongoDB ObjectID (phải là chuỗi hex 24 ký tự)", id),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Validate quyền với document hiện tại trước khi update
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Parse input thành map để chỉ update các trường được chỉ định
		var updateData map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&updateData); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu cập nhật phải là một object JSON hợp lệ. Chi tiết lỗi: %v", err),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép update với validation quyền
		if newOwnerOrgIDStr, ok := updateData["ownerOrganizationId"].(string); ok && newOwnerOrgIDStr != "" {
			// Parse ObjectID
			newOwnerOrgID, err := primitive.ObjectIDFromHex(newOwnerOrgIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"ownerOrganizationId không hợp lệ",
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			// Validate user có quyền với organization mới
			if err := h.validateUserHasAccessToOrg(c, newOwnerOrgID); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}

			// ✅ Có quyền cả 2 (document hiện tại + organization mới) → Thay thế string bằng ObjectID trong updateData để MongoDB lưu đúng kiểu
			updateData["ownerOrganizationId"] = newOwnerOrgID
		} else {
			// Không có ownerOrganizationId trong update → Xóa nếu có (giữ nguyên logic cũ)
			delete(updateData, "ownerOrganizationId")
		}

		// Tạo update data với $set operator
		update := &services.UpdateData{
			Set: updateData,
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		data, err := h.BaseService.UpdateById(ctx, utility.String2ObjectID(id), update)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// DeleteOne xóa một document theo điều kiện filter.
// Filter được truyền qua query string dưới dạng JSON.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) DeleteOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		err = h.BaseService.DeleteOne(c.Context(), filter)
		h.HandleResponse(c, nil, err)
		return nil
	})
}

// DeleteMany xóa nhiều document theo điều kiện filter.
// Filter được truyền qua query string dưới dạng JSON.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có và số lượng document đã xóa
func (h *BaseHandler[T, CreateInput, UpdateInput]) DeleteMany(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		count, err := h.BaseService.DeleteMany(c.Context(), filter)
		h.HandleResponse(c, count, err)
		return nil
	})
}

// DeleteById xóa một document theo ID.
// ID được truyền qua URI params.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) DeleteById(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		id := c.Params("id")
		if id == "" {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"ID không được để trống trong URL params",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		if !primitive.IsValidObjectID(id) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ID '%s' không đúng định dạng MongoDB ObjectID (phải là chuỗi hex 24 ký tự)", id),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Lưu userID vào context để service có thể check admin
		ctx := c.Context()
		if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
			if userID, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
				ctx = services.SetUserIDToContext(ctx, userID)
			}
		}

		err := h.BaseService.DeleteById(ctx, utility.String2ObjectID(id))
		h.HandleResponse(c, nil, err)
		return nil
	})
}

// FindOneAndUpdate tìm và cập nhật một document.
// Filter được truyền qua query string, dữ liệu cập nhật trong request body.
// Trả về document sau khi cập nhật.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOneAndUpdate(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Parse input thành map để chỉ update các trường được chỉ định
		var updateData map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&updateData); err != nil {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu cập nhật không hợp lệ", common.StatusBadRequest, nil))
			return nil
		}

		// Tạo update data với $set operator
		update := &services.UpdateData{
			Set: updateData,
		}

		data, err := h.BaseService.FindOneAndUpdate(c.Context(), filter, update, nil)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// FindOneAndDelete tìm và xóa một document.
// Filter được truyền qua query string.
// Trả về document đã xóa.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOneAndDelete(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		data, err := h.BaseService.FindOneAndDelete(c.Context(), filter, nil)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// CountDocuments đếm số lượng document theo điều kiện filter.
// Filter được truyền qua query string dưới dạng JSON.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) CountDocuments(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var filter map[string]interface{}
		// Lấy giá trị filter từ query string, mặc định là "{}" nếu không có
		filterStr := c.Query("filter", "{}")

		// Log giá trị filter để debug (chỉ log ở level Debug)
		logrus.WithFields(logrus.Fields{
			"filter_string": filterStr,
			"endpoint":      c.Path(),
		}).Debug("Filter string từ query")

		// Chuyển đổi chuỗi JSON thành map
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Log lỗi để debug
			logrus.WithFields(logrus.Fields{
				"filter_string": filterStr,
				"endpoint":      c.Path(),
				"error":         err,
			}).Debug("Lỗi khi parse filter")

			// Trả về lỗi cho client
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				"Filter không hợp lệ",
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Log filter sau khi parse thành công (chỉ log ở level Debug)
		logrus.WithFields(logrus.Fields{
			"filter":   filter,
			"endpoint": c.Path(),
		}).Debug("Filter sau khi parse")

		count, err := h.BaseService.CountDocuments(c.Context(), filter)
		h.HandleResponse(c, count, err)
		return nil
	})
}

// Distinct lấy danh sách giá trị duy nhất của một trường.
// Tên trường được truyền qua URI params, filter qua query string.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) Distinct(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		field := c.Params("field")
		if field == "" {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Tên trường không hợp lệ", common.StatusBadRequest, nil))
			return nil
		}

		var filter map[string]interface{}
		if err := json.Unmarshal([]byte(c.Query("filter", "{}")), &filter); err != nil {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Filter không hợp lệ", common.StatusBadRequest, nil))
			return nil
		}

		data, err := h.BaseService.Distinct(c.Context(), field, filter)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// Upsert thêm mới hoặc cập nhật một document.
// Filter được truyền qua query string, dữ liệu trong request body.
// Nếu không tìm thấy document thỏa mãn filter sẽ tạo mới, ngược lại sẽ cập nhật.
// Parse body thành struct T để struct tag `extract` có thể hoạt động tự động
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) Upsert(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse filter từ query string (sử dụng processFilter để có normalizeFilter và validate)
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		// Parse request body thành struct T (model) để struct tag `extract` có thể hoạt động
		// Struct tag `extract` sẽ tự động extract dữ liệu từ PanCakeData, FacebookData, etc.
		input := new(T)
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
			}
		}

		// Gọi Upsert với struct T - extract sẽ tự động chạy trong ToMap() khi ToUpdateData() được gọi
		data, err := h.BaseService.Upsert(c.Context(), filter, *input)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// UpsertMany thêm mới hoặc cập nhật nhiều document.
// Filter được truyền qua query string, dữ liệu trong request body dưới dạng mảng.
// Với mỗi item trong mảng: nếu không tìm thấy document thỏa mãn filter sẽ tạo mới, ngược lại sẽ cập nhật.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpsertMany(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse filter từ query string (sử dụng processFilter để có normalizeFilter và validate)
		filter, err := h.processFilter(c)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)

		var inputs []T
		if err := h.ParseRequestBody(c, &inputs); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// ✅ Xử lý ownerOrganizationId cho tất cả items: Cho phép chỉ định từ request hoặc dùng context
		for i := range inputs {
			ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(&inputs[i])
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
					h.setOrganizationID(&inputs[i], *activeOrgID)
				}
			}
		}

		// Convert filter từ bson.M sang map[string]interface{} cho UpsertMany
		filterMap := make(map[string]interface{})
		if filter != nil {
			for k, v := range filter {
				filterMap[k] = v
			}
		}

		data, err := h.BaseService.UpsertMany(c.Context(), filterMap, inputs)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// DocumentExists kiểm tra document có tồn tại không.
// Filter được truyền qua query string dưới dạng JSON.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - error: Lỗi nếu có
func (h *BaseHandler[T, CreateInput, UpdateInput]) DocumentExists(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var filter map[string]interface{}
		if err := json.Unmarshal([]byte(c.Query("filter", "{}")), &filter); err != nil {
			h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "Filter không hợp lệ", common.StatusBadRequest, nil))
			return nil
		}

		exists, err := h.BaseService.DocumentExists(c.Context(), filter)
		h.HandleResponse(c, exists, err)
		return nil
	})
}
