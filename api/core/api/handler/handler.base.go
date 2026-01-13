package handler

// Package handler chứa các handler xử lý request HTTP trong ứng dụng.
// Package này cung cấp các chức năng CRUD cơ bản và các tiện ích để xử lý request/response.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
	"meta_commerce/core/utility"
	"reflect"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// ====================================
// ORGANIZATION HELPER FUNCTIONS
// ====================================

// hasOrganizationIDField kiểm tra model có field OwnerOrganizationID không (dùng reflection)
// Field này dùng cho phân quyền dữ liệu (data authorization) - xác định dữ liệu thuộc về tổ chức nào
func (h *BaseHandler[T, CreateInput, UpdateInput]) hasOrganizationIDField() bool {
	var zero T
	val := reflect.ValueOf(zero)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false
	}

	// Tìm field OwnerOrganizationID (tên mới cho phân quyền dữ liệu)
	field := val.FieldByName("OwnerOrganizationID")
	return field.IsValid()
}

// getActiveOrganizationID lấy active organization ID từ context
func (h *BaseHandler[T, CreateInput, UpdateInput]) getActiveOrganizationID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &orgID
}

// setOrganizationID tự động gán ownerOrganizationId vào model (dùng reflection)
// CHỈ gán nếu model có field OwnerOrganizationID
// CHỈ gán từ context nếu model chưa có giá trị (zero) - ưu tiên giá trị từ request body
// **LƯU Ý**: CHỈ set OwnerOrganizationID (phân quyền), KHÔNG set OrganizationID (logic business)
// OrganizationID phải được set riêng từ request body hoặc logic business
func (h *BaseHandler[T, CreateInput, UpdateInput]) setOrganizationID(model interface{}, orgID primitive.ObjectID) {
	// Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return // Model không có OwnerOrganizationID, không cần gán
	}

	// Kiểm tra organizationId không phải zero value
	if orgID.IsZero() {
		return // Không gán zero ObjectID
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName("OwnerOrganizationID")
	if !field.IsValid() || !field.CanSet() {
		return
	}

	// Kiểm tra xem model đã có organizationId chưa (không phải zero)
	// Nếu đã có giá trị hợp lệ từ request body thì không override
	if field.Kind() == reflect.Ptr {
		// Field là pointer
		if !field.IsNil() {
			currentOrgIDPtr := field.Interface().(*primitive.ObjectID)
			if currentOrgIDPtr != nil && !currentOrgIDPtr.IsZero() {
				return // Đã có giá trị hợp lệ, không override
			}
		}
		// Chỉ gán nếu chưa có giá trị hoặc là zero
		field.Set(reflect.ValueOf(&orgID))
	} else {
		// Field là value
		currentOrgID := field.Interface().(primitive.ObjectID)
		if !currentOrgID.IsZero() {
			return // Đã có giá trị hợp lệ từ request body, không override
		}
		// Chỉ gán nếu là zero value
		field.Set(reflect.ValueOf(orgID))
	}
}

// getOrganizationIDFromModel lấy ownerOrganizationId từ model (dùng reflection)
func (h *BaseHandler[T, CreateInput, UpdateInput]) getOrganizationIDFromModel(model T) *primitive.ObjectID {
	// Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return nil // Model không có OwnerOrganizationID
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName("OwnerOrganizationID")
	if !field.IsValid() {
		return nil
	}

	// Xử lý cả primitive.ObjectID và *primitive.ObjectID
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		orgID := field.Interface().(*primitive.ObjectID)
		return orgID
	} else {
		orgID := field.Interface().(primitive.ObjectID)
		return &orgID
	}
}

// getPermissionNameFromRoute lấy permission name từ context (đã được set bởi middleware)
// Middleware AuthMiddleware đã lưu permission name vào context với key "permission_name"
func (h *BaseHandler[T, CreateInput, UpdateInput]) getPermissionNameFromRoute(c fiber.Ctx) string {
	// Lấy từ context (đã được middleware set)
	if permissionName, ok := c.Locals("permission_name").(string); ok && permissionName != "" {
		return permissionName
	}
	return ""
}

// getOwnerOrganizationIDFromModel lấy ownerOrganizationId từ model (dùng reflection)
// Tương tự getOrganizationIDFromModel nhưng tên rõ ràng hơn
func (h *BaseHandler[T, CreateInput, UpdateInput]) getOwnerOrganizationIDFromModel(model interface{}) *primitive.ObjectID {
	// Sử dụng lại logic của getOrganizationIDFromModel
	// Vì getOrganizationIDFromModel đã lấy từ OwnerOrganizationID field
	if !h.hasOrganizationIDField() {
		return nil
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName("OwnerOrganizationID")
	if !field.IsValid() {
		return nil
	}

	// Xử lý cả primitive.ObjectID và *primitive.ObjectID
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		orgID := field.Interface().(*primitive.ObjectID)
		if orgID != nil && !orgID.IsZero() {
			return orgID
		}
	} else {
		orgID := field.Interface().(primitive.ObjectID)
		if !orgID.IsZero() {
			return &orgID
		}
	}

	return nil
}

// validateUserHasAccessToOrg validate user có quyền với organization không
// Dùng để validate khi create/update với ownerOrganizationId từ request
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateUserHasAccessToOrg(c fiber.Ctx, orgID primitive.ObjectID) error {
	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return common.NewError(common.ErrCodeAuthRole, "Không có role context", common.StatusUnauthorized, nil)
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return common.NewError(common.ErrCodeAuthRole, "Role ID không hợp lệ", common.StatusUnauthorized, err)
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
	if err != nil {
		return err
	}

	// Kiểm tra organization có trong allowed list không
	for _, allowedOrgID := range allowedOrgIDs {
		if allowedOrgID == orgID {
			return nil // ✅ Có quyền
		}
	}

	// ❌ Không có quyền
	return common.NewError(
		common.ErrCodeAuthRole,
		"Không có quyền với organization này",
		common.StatusForbidden,
		nil,
	)
}

// applyOrganizationFilter tự động thêm filter ownerOrganizationId
// CHỈ áp dụng nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(c fiber.Ctx, baseFilter bson.M) bson.M {
	// ✅ QUAN TRỌNG: Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return baseFilter // Model không có OwnerOrganizationID, không cần filter
	}

	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return baseFilter // Không có active role, không filter
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return baseFilter
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
	if err != nil || len(allowedOrgIDs) == 0 {
		return baseFilter
	}

	// Lấy organizations được share với user's organizations
	sharedOrgIDs, err := services.GetSharedOrganizationIDs(c.Context(), allowedOrgIDs, permissionName)
	if err == nil && len(sharedOrgIDs) > 0 {
		// Hợp nhất allowedOrgIDs và sharedOrgIDs
		allOrgIDsMap := make(map[primitive.ObjectID]bool)
		for _, orgID := range allowedOrgIDs {
			allOrgIDsMap[orgID] = true
		}
		for _, orgID := range sharedOrgIDs {
			allOrgIDsMap[orgID] = true
		}

		// Convert back to slice
		allOrgIDs := make([]primitive.ObjectID, 0, len(allOrgIDsMap))
		for orgID := range allOrgIDsMap {
			allOrgIDs = append(allOrgIDs, orgID)
		}
		allowedOrgIDs = allOrgIDs
	}

	// Thêm filter ownerOrganizationId (phân quyền dữ liệu)
	orgFilter := bson.M{"ownerOrganizationId": bson.M{"$in": allowedOrgIDs}}

	// Kết hợp với baseFilter
	if len(baseFilter) == 0 {
		return orgFilter
	}

	return bson.M{
		"$and": []bson.M{
			baseFilter,
			orgFilter,
		},
	}
}

// validateOrganizationAccess validate user có quyền truy cập document này không
// CHỈ validate nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateOrganizationAccess(c fiber.Ctx, documentID string) error {
	// ✅ QUAN TRỌNG: Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return nil // Model không có OwnerOrganizationID, không cần validate
	}

	// Lấy document
	id, err := primitive.ObjectIDFromHex(documentID)
	if err != nil {
		return common.NewError(common.ErrCodeValidationInput, "ID không hợp lệ", common.StatusBadRequest, err)
	}

	doc, err := h.BaseService.FindOneById(c.Context(), id)
	if err != nil {
		return err
	}

	// Lấy organizationId từ document (dùng reflection)
	docOrgID := h.getOrganizationIDFromModel(doc)
	if docOrgID == nil {
		return nil // Không có organizationId, không cần validate
	}

	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return common.NewError(common.ErrCodeAuthRole, "Không có role context", common.StatusUnauthorized, nil)
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return common.NewError(common.ErrCodeAuthRole, "Role ID không hợp lệ", common.StatusUnauthorized, err)
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
	if err != nil {
		return err
	}

	// Kiểm tra document có thuộc allowed organizations không
	for _, allowedOrgID := range allowedOrgIDs {
		if allowedOrgID == *docOrgID {
			return nil // Có quyền truy cập
		}
	}

	return common.NewError(common.ErrCodeAuthRole, "Không có quyền truy cập", common.StatusForbidden, nil)
}

// FilterOptions cấu hình cho việc validate filter
type FilterOptions struct {
	DeniedFields     []string // Các trường bị cấm filter
	AllowedOperators []string // Các operator MongoDB được phép
	MaxFields        int      // Số lượng field tối đa trong một filter
}

// BaseHandler là base handler cho các Fiber handler, cung cấp các chức năng CRUD cơ bản.
// Struct này sử dụng Generic Type để có thể tái sử dụng cho nhiều loại model khác nhau.
//
// Type parameters:
// - T: Kiểu dữ liệu của model
// - CreateInput: Kiểu dữ liệu của input khi tạo mới
// - UpdateInput: Kiểu dữ liệu của input khi cập nhật
type BaseHandler[T any, CreateInput any, UpdateInput any] struct {
	BaseService   services.BaseServiceMongo[T] // Service xử lý logic nghiệp vụ với MongoDB
	filterOptions FilterOptions                // Cấu hình validate filter
}

// NewBaseHandler tạo mới một BaseHandler với BaseService được cung cấp
func NewBaseHandler[T any, CreateInput any, UpdateInput any](baseService services.BaseServiceMongo[T]) *BaseHandler[T, CreateInput, UpdateInput] {
	return &BaseHandler[T, CreateInput, UpdateInput]{
		BaseService: baseService,
		filterOptions: FilterOptions{
			DeniedFields: []string{
				"password",
				"token",
				"secret",
				"key",
				"hash",
			},
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
		},
	}
}

// validateInput thực hiện validate chi tiết dữ liệu đầu vào
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateInput(input interface{}) error {
	// Validate với validator từ global
	if err := global.Validate.Struct(input); err != nil {
		return common.NewError(common.ErrCodeValidationInput, common.MsgValidationError, common.StatusBadRequest, err)
	}

	// Kiểm tra các trường đặc biệt
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Chỉ xử lý nếu input là struct
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Kiểm tra các trường string
		if field.Kind() == reflect.String {
			// Kiểm tra độ dài tối đa (nếu có tag maxLength)
			if maxTag := fieldType.Tag.Get("maxLength"); maxTag != "" {
				maxLen, err := strconv.Atoi(maxTag)
				if err == nil && len(field.String()) > maxLen {
					return common.NewError(
						common.ErrCodeValidationInput,
						fmt.Sprintf("Trường %s vượt quá độ dài cho phép (%d ký tự)", fieldType.Name, maxLen),
						common.StatusBadRequest,
						nil,
					)
				}
			}
		}

		// Kiểm tra các trường số
		if field.Kind() == reflect.Int || field.Kind() == reflect.Int64 {
			// Kiểm tra giá trị tối thiểu (nếu có tag min)
			if minTag := fieldType.Tag.Get("min"); minTag != "" {
				min, err := strconv.ParseInt(minTag, 10, 64)
				if err == nil && field.Int() < min {
					return common.NewError(
						common.ErrCodeValidationInput,
						fmt.Sprintf("Trường %s phải lớn hơn hoặc bằng %d", fieldType.Name, min),
						common.StatusBadRequest,
						nil,
					)
				}
			}

			// Kiểm tra giá trị tối đa (nếu có tag max)
			if maxTag := fieldType.Tag.Get("max"); maxTag != "" {
				max, err := strconv.ParseInt(maxTag, 10, 64)
				if err == nil && field.Int() > max {
					return common.NewError(
						common.ErrCodeValidationInput,
						fmt.Sprintf("Trường %s phải nhỏ hơn hoặc bằng %d", fieldType.Name, max),
						common.StatusBadRequest,
						nil,
					)
				}
			}
		}
	}

	return nil
}

// ParseRequestBody parse và validate dữ liệu từ request body.
// Sử dụng json.Decoder với UseNumber() để xử lý chính xác các số.
//
// Parameters:
// - c: Fiber context
// - input: Con trỏ tới struct sẽ chứa dữ liệu được parse
//
// Returns:
// - error: Lỗi nếu có trong quá trình parse hoặc validate
func (h *BaseHandler[T, CreateInput, UpdateInput]) ParseRequestBody(c fiber.Ctx, input interface{}) error {
	// Parse body thành struct T
	body := c.Body()
	reader := bytes.NewReader(body)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(input); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, common.MsgValidationError, common.StatusBadRequest, err)
	}

	// Validate chi tiết input
	if err := h.validateInput(input); err != nil {
		return err
	}

	return nil
}

// ParseRequestQuery parse và validate dữ liệu từ query string.
// Query string phải được encode dưới dạng JSON.
//
// Parameters:
// - c: Fiber context
// - input: Con trỏ tới struct sẽ chứa dữ liệu được parse
//
// Returns:
// - error: Lỗi nếu có trong quá trình parse hoặc validate
func (h *BaseHandler[T, CreateInput, UpdateInput]) ParseRequestQuery(c fiber.Ctx, input interface{}) error {
	query := c.Query("query", "")

	// Parse query
	reader := bytes.NewReader([]byte(query))
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(input); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, common.MsgValidationError, common.StatusBadRequest, err)
	}

	// Validate struct
	if err := global.Validate.Struct(input); err != nil {
		return common.NewError(common.ErrCodeValidationInput, common.MsgValidationError, common.StatusBadRequest, err)
	}

	return nil
}

// ParseRequestParams parse và validate các tham số từ URI.
// Sử dụng Fiber's URI binding để parse các tham số.
//
// Parameters:
// - c: Fiber context
// - input: Con trỏ tới struct sẽ chứa dữ liệu được parse
//
// Returns:
// - error: Lỗi nếu có trong quá trình parse hoặc validate
func (h *BaseHandler[T, CreateInput, UpdateInput]) ParseRequestParams(c fiber.Ctx, input interface{}) error {
	// Parse URI params
	if err := c.Bind().URI(input); err != nil {
		return common.NewError(common.ErrCodeValidationFormat, common.MsgValidationError, common.StatusBadRequest, err)
	}

	// Validate struct
	if err := global.Validate.Struct(input); err != nil {
		return common.NewError(common.ErrCodeValidationInput, common.MsgValidationError, common.StatusBadRequest, err)
	}

	return nil
}

// processFilter xử lý và validate filter từ request
func (h *BaseHandler[T, CreateInput, UpdateInput]) processFilter(c fiber.Ctx) (map[string]interface{}, error) {
	var filter map[string]interface{}

	// Parse filter từ query
	filterStr := c.Query("filter", "{}")
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		return nil, common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Filter không đúng định dạng JSON. Chi tiết lỗi: %v. Giá trị filter nhận được: %s", err, filterStr),
			common.StatusBadRequest,
			err,
		)
	}

	// Normalize filter: chuyển đổi các string ObjectId thành ObjectID
	filter = h.normalizeFilter(filter)

	// Validate filter
	if err := h.validateFilter(filter); err != nil {
		return nil, err
	}

	return filter, nil
}

// normalizeFilter chuyển đổi các string có format ObjectId thành ObjectID trong filter
// Hỗ trợ các trường có tên kết thúc bằng "Id" hoặc "ID"
func (h *BaseHandler[T, CreateInput, UpdateInput]) normalizeFilter(filter map[string]interface{}) map[string]interface{} {
	if filter == nil {
		return filter
	}

	normalized := make(map[string]interface{})
	for field, value := range filter {
		// Kiểm tra nếu field name kết thúc bằng "Id" hoặc "ID" (case-insensitive)
		fieldLower := strings.ToLower(field)
		isIDField := strings.HasSuffix(fieldLower, "id") && len(fieldLower) > 2

		normalized[field] = h.normalizeFilterValue(value, isIDField)
	}

	return normalized
}

// normalizeFilterValue chuyển đổi giá trị trong filter, hỗ trợ nested structures
func (h *BaseHandler[T, CreateInput, UpdateInput]) normalizeFilterValue(value interface{}, isIDField bool) interface{} {
	if value == nil {
		return value
	}

	// Hỗ trợ MongoDB Extended JSON format: {"$oid": "..."}
	if mapValue, ok := value.(map[string]interface{}); ok {
		if oidValue, hasOid := mapValue["$oid"]; hasOid {
			if oidStr, ok := oidValue.(string); ok {
				if primitive.IsValidObjectID(oidStr) {
					objID, err := primitive.ObjectIDFromHex(oidStr)
					if err == nil {
						return objID
					}
				}
			}
			// Nếu $oid không hợp lệ, trả về giá trị gốc
			return value
		}
	}

	// Nếu là string và field là ID field, thử chuyển đổi thành ObjectID
	if strValue, ok := value.(string); ok && isIDField {
		if primitive.IsValidObjectID(strValue) {
			objID, err := primitive.ObjectIDFromHex(strValue)
			if err == nil {
				return objID
			}
		}
		return strValue
	}

	// Nếu là mảng, xử lý từng phần tử
	if arrValue, ok := value.([]interface{}); ok {
		normalizedArr := make([]interface{}, len(arrValue))
		for i, item := range arrValue {
			normalizedArr[i] = h.normalizeFilterValue(item, isIDField)
		}
		return normalizedArr
	}

	// Nếu là map (cho các operator như $in, $nin, $eq, etc.), xử lý đệ quy
	if mapValue, ok := value.(map[string]interface{}); ok {
		normalizedMap := make(map[string]interface{})
		for key, val := range mapValue {
			// Đặc biệt xử lý $in và $nin - các giá trị trong mảng cần được chuyển đổi
			if (key == "$in" || key == "$nin") && isIDField {
				if arrVal, ok := val.([]interface{}); ok {
					normalizedArr := make([]interface{}, len(arrVal))
					for i, item := range arrVal {
						if strItem, ok := item.(string); ok && primitive.IsValidObjectID(strItem) {
							objID, err := primitive.ObjectIDFromHex(strItem)
							if err == nil {
								normalizedArr[i] = objID
							} else {
								normalizedArr[i] = item
							}
						} else {
							normalizedArr[i] = item
						}
					}
					normalizedMap[key] = normalizedArr
				} else {
					normalizedMap[key] = val
				}
			} else {
				// Xử lý các operator khác như $eq
				normalizedMap[key] = h.normalizeFilterValue(val, isIDField)
			}
		}
		return normalizedMap
	}

	return value
}

// validateFilter kiểm tra tính hợp lệ của filter
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateFilter(filter map[string]interface{}) error {
	// Khởi tạo giá trị mặc định nếu filterOptions chưa được khởi tạo đúng cách
	deniedFields := h.filterOptions.DeniedFields
	if len(deniedFields) == 0 {
		deniedFields = []string{
			"password",
			"token",
			"secret",
			"key",
			"hash",
		}
	}

	allowedOperators := h.filterOptions.AllowedOperators
	if len(allowedOperators) == 0 {
		allowedOperators = []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		}
	}

	maxFields := h.filterOptions.MaxFields
	if maxFields == 0 {
		maxFields = 10 // Giá trị mặc định
	}

	// Kiểm tra số lượng field
	if len(filter) > maxFields {
		return common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Filter vượt quá số lượng trường cho phép. Tối đa %d trường, hiện tại có %d trường. Vui lòng giảm số lượng trường trong filter.", maxFields, len(filter)),
			common.StatusBadRequest,
			nil,
		)
	}

	// Kiểm tra từng field và operator
	for field, value := range filter {
		// Kiểm tra field có bị cấm không
		if utility.Contains(deniedFields, field) {
			return common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Trường '%s' không được phép sử dụng trong filter vì lý do bảo mật. Vui lòng sử dụng các trường khác.", field),
				common.StatusBadRequest,
				nil,
			)
		}

		// Kiểm tra operator nếu value là map
		if mapValue, ok := value.(map[string]interface{}); ok {
			for op := range mapValue {
				if strings.HasPrefix(op, "$") && !utility.Contains(allowedOperators, op) {
					return common.NewError(
						common.ErrCodeValidationFormat,
						fmt.Sprintf("Toán tử MongoDB '%s' không được phép sử dụng. Các toán tử được phép: %v", op, allowedOperators),
						common.StatusBadRequest,
						nil,
					)
				}
			}
		}
	}

	return nil
}

// processMongoOptions xử lý options từ query string và chuyển đổi sang MongoDB options
func (h *BaseHandler[T, CreateInput, UpdateInput]) processMongoOptions(c fiber.Ctx, isFindOne bool) (interface{}, error) {
	var rawOptions map[string]interface{}

	// Parse options từ query string
	optionsStr := c.Query("options", "{}")
	if err := json.Unmarshal([]byte(optionsStr), &rawOptions); err != nil {
		return nil, common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Options không đúng định dạng JSON. Chi tiết lỗi: %v. Giá trị options nhận được: %s", err, optionsStr),
			common.StatusBadRequest,
			err,
		)
	}

	// Validate options
	if err := h.validateMongoOptions(rawOptions); err != nil {
		return nil, err
	}

	// Helper function để parse sort map thông thường (fallback)
	parseSortMap := func(sortMap map[string]interface{}) bson.D {
		sortBson := bson.D{}
		for field, value := range sortMap {
			var sortValue int
			if v, ok := value.(float64); ok {
				sortValue = int(v)
			} else if v, ok := value.(int); ok {
				sortValue = v
			} else {
				continue
			}
			// Validate sort value
			if sortValue != 1 && sortValue != -1 {
				continue
			}
			sortBson = append(sortBson, bson.E{Key: field, Value: sortValue})
		}
		return sortBson
	}

	// Parse sort với thứ tự được giữ nguyên từ JSON string gốc
	parseSortWithOrder := func(sortMap map[string]interface{}, optionsJSON string) (bson.D, error) {
		sortBson := bson.D{}

		// Parse lại phần sort từ JSON string gốc để giữ nguyên thứ tự
		// Sử dụng json.Decoder với Token() để parse từng key theo thứ tự trong JSON
		var tempOptions map[string]json.RawMessage
		if err := json.Unmarshal([]byte(optionsJSON), &tempOptions); err != nil {
			// Fallback: sử dụng map thông thường nếu không parse được
			return parseSortMap(sortMap), nil
		}

		sortRaw, ok := tempOptions["sort"]
		if !ok {
			// Không có sort trong options
			return sortBson, nil
		}

		// Parse sort object với json.Decoder để giữ thứ tự các key
		decoder := json.NewDecoder(bytes.NewReader(sortRaw))
		decoder.UseNumber() // Sử dụng Number để tránh mất precision

		// Đọc token '{'
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			// Fallback: sử dụng map thông thường
			return parseSortMap(sortMap), nil
		}

		// Parse từng key-value pair theo thứ tự trong JSON
		for decoder.More() {
			// Đọc key
			keyToken, err := decoder.Token()
			if err != nil {
				break
			}
			field, ok := keyToken.(string)
			if !ok {
				continue
			}

			// Đọc value token (số)
			valueToken, err := decoder.Token()
			if err != nil {
				break
			}

			var sortValue int
			switch v := valueToken.(type) {
			case json.Number:
				intVal, err := v.Int64()
				if err != nil {
					// Thử parse như float64
					floatVal, err := v.Float64()
					if err != nil {
						continue
					}
					intVal = int64(floatVal)
				}
				sortValue = int(intVal)
			case float64:
				sortValue = int(v)
			case int:
				sortValue = v
			default:
				continue
			}

			// Validate sort value (chỉ chấp nhận 1 hoặc -1)
			if sortValue != 1 && sortValue != -1 {
				continue
			}

			sortBson = append(sortBson, bson.E{Key: field, Value: sortValue})
		}

		// Đọc token '}'
		_, _ = decoder.Token()

		// Nếu không parse được gì, fallback về map thông thường
		if len(sortBson) == 0 {
			return parseSortMap(sortMap), nil
		}

		return sortBson, nil
	}

	// Chuyển đổi sang MongoDB options
	if isFindOne {
		opts := mongoopts.FindOne()
		if projection, ok := rawOptions["projection"].(map[string]interface{}); ok {
			opts.SetProjection(projection)
		}
		if sort, ok := rawOptions["sort"].(map[string]interface{}); ok {
			sortBson, err := parseSortWithOrder(sort, optionsStr)
			if err != nil {
				return nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("Lỗi khi parse sort options: %v", err),
					common.StatusBadRequest,
					err,
				)
			}
			opts.SetSort(sortBson)
		}
		return opts, nil
	}

	opts := mongoopts.Find()
	if projection, ok := rawOptions["projection"].(map[string]interface{}); ok {
		opts.SetProjection(projection)
	}
	if sort, ok := rawOptions["sort"].(map[string]interface{}); ok {
		sortBson, err := parseSortWithOrder(sort, optionsStr)
		if err != nil {
			return nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Lỗi khi parse sort options: %v", err),
				common.StatusBadRequest,
				err,
			)
		}
		opts.SetSort(sortBson)
	}
	if limit, ok := rawOptions["limit"].(float64); ok {
		opts.SetLimit(int64(limit))
	}
	if skip, ok := rawOptions["skip"].(float64); ok {
		opts.SetSkip(int64(skip))
	}
	return opts, nil
}

// validateMongoOptions kiểm tra tính hợp lệ của các options
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateMongoOptions(options map[string]interface{}) error {
	// Khởi tạo giá trị mặc định nếu filterOptions chưa được khởi tạo đúng cách
	deniedFields := h.filterOptions.DeniedFields
	if len(deniedFields) == 0 {
		deniedFields = []string{
			"password",
			"token",
			"secret",
			"key",
			"hash",
		}
	}

	// Danh sách các options được phép
	allowedOptions := map[string]bool{
		"projection": true,
		"sort":       true,
		"limit":      true,
		"skip":       true,
	}

	// Kiểm tra các options không hợp lệ
	for key := range options {
		if !allowedOptions[key] {
			return common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Option '%s' không được hỗ trợ. Các options được phép: projection, sort, limit, skip", key),
				common.StatusBadRequest,
				nil,
			)
		}
	}

	// Validate projection
	if projection, ok := options["projection"].(map[string]interface{}); ok {
		for field := range projection {
			// Kiểm tra các trường bị cấm
			if utility.Contains(deniedFields, field) {
				return common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("Trường '%s' không được phép sử dụng trong projection vì lý do bảo mật", field),
					common.StatusBadRequest,
					nil,
				)
			}
		}
	}

	// Validate sort
	if sort, ok := options["sort"].(map[string]interface{}); ok {
		for field, value := range sort {
			// Kiểm tra các trường bị cấm
			if utility.Contains(deniedFields, field) {
				return common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("Trường '%s' không được phép sử dụng trong sort vì lý do bảo mật", field),
					common.StatusBadRequest,
					nil,
				)
			}
			// Kiểm tra giá trị sort (1 hoặc -1)
			if v, ok := value.(float64); !ok || (v != 1 && v != -1) {
				return common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("Giá trị sort cho trường '%s' phải là 1 (tăng dần) hoặc -1 (giảm dần), giá trị hiện tại: %v", field, value),
					common.StatusBadRequest,
					nil,
				)
			}
		}
	}

	// Validate limit
	if limit, ok := options["limit"].(float64); ok {
		if limit <= 0 {
			return common.NewError(
				common.ErrCodeValidationFormat,
				"Giá trị limit phải lớn hơn 0",
				common.StatusBadRequest,
				nil,
			)
		}
		if limit > 1000 {
			return common.NewError(
				common.ErrCodeValidationFormat,
				"Giá trị limit không được vượt quá 1000 để đảm bảo hiệu năng hệ thống",
				common.StatusBadRequest,
				nil,
			)
		}
	}

	// Validate skip
	if skip, ok := options["skip"].(float64); ok {
		if skip < 0 {
			return common.NewError(
				common.ErrCodeValidationFormat,
				"Giá trị skip không được âm",
				common.StatusBadRequest,
				nil,
			)
		}
	}

	return nil
}

// ParsePagination xử lý việc parse thông tin phân trang từ request.
// Hỗ trợ các tham số:
// - page: Số trang (mặc định: 1)
// - limit: Số lượng item trên một trang (mặc định: 10)
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - page: Số trang
// - limit: Số lượng item trên một trang
func (h *BaseHandler[T, CreateInput, UpdateInput]) ParsePagination(c fiber.Ctx) (int64, int64) {
	page := utility.P2Int64(c.Query("page", "1"))
	if page <= 0 {
		page = 1
	}

	limit := utility.P2Int64(c.Query("limit", "10"))
	if limit <= 0 {
		limit = 10
	}

	return page, limit
}

// GetIDFromContext lấy ID từ URI params của request.
//
// Parameters:
// - c: Fiber context
//
// Returns:
// - string: ID từ params
func (h *BaseHandler[T, CreateInput, UpdateInput]) GetIDFromContext(c fiber.Ctx) string {
	return c.Params("id")
}

// transformCreateInputToModel transform CreateInput (DTO) sang Model (T)
// Sử dụng struct tag `transform` để tự động convert các field (ví dụ: string → ObjectID)
// Hỗ trợ map field từ DTO sang Model với tên khác nhau thông qua option `map=<field_name>`
//
// Parameters:
// - input: CreateInput (DTO) cần transform
//
// Returns:
// - *T: Model đã được transform
// - error: Lỗi nếu có trong quá trình transform
func (h *BaseHandler[T, CreateInput, UpdateInput]) transformCreateInputToModel(input *CreateInput) (*T, error) {
	// Tạo Model mới
	model := new(T)

	// Lấy reflect value và type của DTO và Model
	inputVal := reflect.ValueOf(input)
	if inputVal.Kind() == reflect.Ptr {
		inputVal = inputVal.Elem()
	}
	if inputVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("CreateInput phải là struct hoặc pointer đến struct")
	}

	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() == reflect.Ptr {
		modelVal = modelVal.Elem()
	}
	if modelVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("Model phải là struct hoặc pointer đến struct")
	}

	inputType := inputVal.Type()
	modelType := modelVal.Type()

	// Duyệt qua tất cả các field trong DTO
	for i := 0; i < inputVal.NumField(); i++ {
		inputField := inputVal.Field(i)
		inputFieldType := inputType.Field(i)

		// Bỏ qua field không export được
		if !inputField.CanInterface() {
			continue
		}

		// Lấy giá trị field
		fieldValue := inputField.Interface()

		// Kiểm tra có transform tag không
		transformTag := inputFieldType.Tag.Get("transform")
		if transformTag != "" {
			// Parse transform tag
			transformConfig, err := utility.ParseTransformTag(transformTag)
			if err != nil {
				return nil, fmt.Errorf("lỗi parse transform tag cho field %s: %w", inputFieldType.Name, err)
			}

			// Xác định field target trong Model
			targetFieldName := inputFieldType.Name // Mặc định: cùng tên
			if transformConfig.MapTo != "" {
				// Có map option → dùng tên field từ map
				targetFieldName = transformConfig.MapTo
			}

			// Tìm field trong Model
			modelField, found := modelType.FieldByName(targetFieldName)
			if !found {
				// Không tìm thấy field trong Model
				if transformConfig.Optional {
					// Optional field → bỏ qua
					continue
				}
				return nil, fmt.Errorf("không tìm thấy field '%s' trong Model (map từ field '%s' trong DTO)", targetFieldName, inputFieldType.Name)
			}

			// Transform giá trị
			transformedValue, err := utility.TransformFieldValue(fieldValue, transformConfig, modelField.Type)
			if err != nil {
				if transformConfig.Optional {
					// Optional field → bỏ qua lỗi
					continue
				}
				return nil, fmt.Errorf("lỗi transform field '%s' sang '%s': %w", inputFieldType.Name, targetFieldName, err)
			}

			// Set giá trị vào Model field
			modelFieldVal := modelVal.FieldByName(targetFieldName)
			if !modelFieldVal.IsValid() || !modelFieldVal.CanSet() {
				return nil, fmt.Errorf("không thể set giá trị vào field '%s' trong Model", targetFieldName)
			}

			// Convert và set giá trị
			if transformedValue != nil {
				transformedVal := reflect.ValueOf(transformedValue)
				if transformedVal.Type().AssignableTo(modelFieldVal.Type()) {
					modelFieldVal.Set(transformedVal)
				} else if transformedVal.Type().ConvertibleTo(modelFieldVal.Type()) {
					modelFieldVal.Set(transformedVal.Convert(modelFieldVal.Type()))
				} else {
					return nil, fmt.Errorf("không thể convert giá trị từ type %v sang type %v cho field '%s'", transformedVal.Type(), modelFieldVal.Type(), targetFieldName)
				}
			} else if transformConfig.Optional {
				// Optional field với giá trị nil → giữ nguyên zero value
				continue
			}
		} else {
			// Không có transform tag → copy trực tiếp nếu field cùng tên và type tương thích
			targetFieldName := inputFieldType.Name
			_, found := modelType.FieldByName(targetFieldName)
			if !found {
				// Không có field cùng tên trong Model → bỏ qua
				continue
			}

			// Kiểm tra type tương thích
			modelFieldVal := modelVal.FieldByName(targetFieldName)
			if !modelFieldVal.IsValid() || !modelFieldVal.CanSet() {
				continue
			}

			// Copy giá trị nếu type tương thích
			inputValReflect := reflect.ValueOf(fieldValue)
			if inputValReflect.Type().AssignableTo(modelFieldVal.Type()) {
				modelFieldVal.Set(inputValReflect)
			} else if inputValReflect.Type().ConvertibleTo(modelFieldVal.Type()) {
				modelFieldVal.Set(inputValReflect.Convert(modelFieldVal.Type()))
			}
			// Nếu không tương thích → bỏ qua (có thể cần transform tag)
		}
	}

	return model, nil
}
