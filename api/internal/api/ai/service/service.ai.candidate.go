package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AICandidateService là service quản lý AI candidates (Module 2)
type AICandidateService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AICandidate]
}

// NewAICandidateService tạo mới AICandidateService
func NewAICandidateService() (*AICandidateService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AICandidates)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_candidates collection: %v", common.ErrNotFound)
	}
	return &AICandidateService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AICandidate](collection),
	}, nil
}
