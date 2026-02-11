package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIProviderProfileService là service quản lý AI provider profiles (Module 2)
type AIProviderProfileService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIProviderProfile]
}

// NewAIProviderProfileService tạo mới AIProviderProfileService
func NewAIProviderProfileService() (*AIProviderProfileService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIProviderProfiles)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_provider_profiles collection: %v", common.ErrNotFound)
	}
	return &AIProviderProfileService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIProviderProfile](collection),
	}, nil
}
