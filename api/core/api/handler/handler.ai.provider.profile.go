package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIProviderProfileHandler xử lý các request liên quan đến AI Provider Profile (Module 2)
type AIProviderProfileHandler struct {
	*BaseHandler[models.AIProviderProfile, dto.AIProviderProfileCreateInput, dto.AIProviderProfileUpdateInput]
	AIProviderProfileService *services.AIProviderProfileService
}

// NewAIProviderProfileHandler tạo mới AIProviderProfileHandler
// Trả về:
//   - *AIProviderProfileHandler: Instance mới của AIProviderProfileHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIProviderProfileHandler() (*AIProviderProfileHandler, error) {
	aiProviderProfileService, err := services.NewAIProviderProfileService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider profile service: %v", err)
	}

	handler := &AIProviderProfileHandler{
		AIProviderProfileService: aiProviderProfileService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIProviderProfile, dto.AIProviderProfileCreateInput, dto.AIProviderProfileUpdateInput](aiProviderProfileService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
// Nested struct Config đã được xử lý tự động bởi transform:"nested_struct" trong DTO
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler

// UpdateOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.UpdateOne trực tiếp
// Nested struct Config đã được xử lý tự động bởi transform:"nested_struct" trong DTO
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler

// Tất cả các CRUD operations khác đã được cung cấp bởi BaseHandler với transform tag tự động
// - UpdateById: Cập nhật AI provider profile
// - FindOneById: Lấy AI provider profile theo ID
// - FindWithPagination: Lấy danh sách AI provider profile với phân trang
// - DeleteById: Xóa AI provider profile theo ID
