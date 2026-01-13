package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AIProviderProfileService là service quản lý AI provider profiles (Module 2)
type AIProviderProfileService struct {
	*BaseServiceMongoImpl[models.AIProviderProfile]
}

// NewAIProviderProfileService tạo mới AIProviderProfileService
// Trả về:
//   - *AIProviderProfileService: Instance mới của AIProviderProfileService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIProviderProfileService() (*AIProviderProfileService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIProviderProfiles)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_provider_profiles collection: %v", common.ErrNotFound)
	}

	return &AIProviderProfileService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIProviderProfile](collection),
	}, nil
}
