package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIRunHandler xử lý các request liên quan đến AI Run (Module 2)
type AIRunHandler struct {
	*BaseHandler[models.AIRun, dto.AIRunCreateInput, dto.AIRunUpdateInput]
	AIRunService *services.AIRunService
}

// NewAIRunHandler tạo mới AIRunHandler
// Trả về:
//   - *AIRunHandler: Instance mới của AIRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIRunHandler() (*AIRunHandler, error) {
	aiRunService, err := services.NewAIRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI run service: %v", err)
	}

	handler := &AIRunHandler{
		AIRunService: aiRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIRun, dto.AIRunCreateInput, dto.AIRunUpdateInput](aiRunService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler
