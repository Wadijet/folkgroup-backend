package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
)

// AICandidateHandler xử lý các request liên quan đến AI Candidate (Module 2)
type AICandidateHandler struct {
	*basehdl.BaseHandler[aimodels.AICandidate, aidto.AICandidateCreateInput, aidto.AICandidateUpdateInput]
	AICandidateService *aisvc.AICandidateService
}

// NewAICandidateHandler tạo mới AICandidateHandler
func NewAICandidateHandler() (*AICandidateHandler, error) {
	aiCandidateService, err := aisvc.NewAICandidateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI candidate service: %v", err)
	}

	hdl := &AICandidateHandler{
		AICandidateService: aiCandidateService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AICandidate, aidto.AICandidateCreateInput, aidto.AICandidateUpdateInput](aiCandidateService.BaseServiceMongoImpl)

	return hdl, nil
}
