package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIPromptTemplateHandler xử lý các request liên quan đến AI Prompt Template (Module 2)
type AIPromptTemplateHandler struct {
	*BaseHandler[models.AIPromptTemplate, dto.AIPromptTemplateCreateInput, dto.AIPromptTemplateUpdateInput]
	AIPromptTemplateService *services.AIPromptTemplateService
}

// NewAIPromptTemplateHandler tạo mới AIPromptTemplateHandler
// Trả về:
//   - *AIPromptTemplateHandler: Instance mới của AIPromptTemplateHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIPromptTemplateHandler() (*AIPromptTemplateHandler, error) {
	aiPromptTemplateService, err := services.NewAIPromptTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI prompt template service: %v", err)
	}

	handler := &AIPromptTemplateHandler{
		AIPromptTemplateService: aiPromptTemplateService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIPromptTemplate, dto.AIPromptTemplateCreateInput, dto.AIPromptTemplateUpdateInput](aiPromptTemplateService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
// Nested struct Provider đã được xử lý tự động bởi transform:"nested_struct" trong DTO
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler

// UpdateOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.UpdateOne trực tiếp
// Nested struct Provider đã được xử lý tự động bởi transform:"nested_struct" trong DTO
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler
