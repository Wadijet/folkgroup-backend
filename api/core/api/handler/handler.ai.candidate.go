package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AICandidateHandler xử lý các request liên quan đến AI Candidate (Module 2)
type AICandidateHandler struct {
	*BaseHandler[models.AICandidate, dto.AICandidateCreateInput, dto.AICandidateUpdateInput]
	AICandidateService *services.AICandidateService
}

// NewAICandidateHandler tạo mới AICandidateHandler
// Trả về:
//   - *AICandidateHandler: Instance mới của AICandidateHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAICandidateHandler() (*AICandidateHandler, error) {
	aiCandidateService, err := services.NewAICandidateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI candidate service: %v", err)
	}

	handler := &AICandidateHandler{
		AICandidateService: aiCandidateService,
	}
	handler.BaseHandler = NewBaseHandler[models.AICandidate, dto.AICandidateCreateInput, dto.AICandidateUpdateInput](aiCandidateService.BaseServiceMongoImpl)

	return handler, nil
}
