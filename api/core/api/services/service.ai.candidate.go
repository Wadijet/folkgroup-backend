package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// AICandidateService là service quản lý AI candidates (Module 2)
type AICandidateService struct {
	*BaseServiceMongoImpl[models.AICandidate]
}

// NewAICandidateService tạo mới AICandidateService
// Trả về:
//   - *AICandidateService: Instance mới của AICandidateService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAICandidateService() (*AICandidateService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AICandidates)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_candidates collection: %v", common.ErrNotFound)
	}

	return &AICandidateService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AICandidate](collection),
	}, nil
}
