package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AIStepRunHandler xử lý các request liên quan đến AI Step Run (Module 2)
type AIStepRunHandler struct {
	*BaseHandler[models.AIStepRun, dto.AIStepRunCreateInput, dto.AIStepRunUpdateInput]
	AIStepRunService *services.AIStepRunService
}

// NewAIStepRunHandler tạo mới AIStepRunHandler
// Trả về:
//   - *AIStepRunHandler: Instance mới của AIStepRunHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepRunHandler() (*AIStepRunHandler, error) {
	aiStepRunService, err := services.NewAIStepRunService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step run service: %v", err)
	}

	handler := &AIStepRunHandler{
		AIStepRunService: aiStepRunService,
	}
	handler.BaseHandler = NewBaseHandler[models.AIStepRun, dto.AIStepRunCreateInput, dto.AIStepRunUpdateInput](aiStepRunService.BaseServiceMongoImpl)

	return handler, nil
}

// InsertOne: KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
// Tất cả logic (validation, transform, ownerOrganizationId, timestamps) đã được xử lý tự động bởi BaseHandler
