package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
)

// DraftApprovalService là service quản lý draft approvals
type DraftApprovalService struct {
	*BaseServiceMongoImpl[models.DraftApproval]
}

// NewDraftApprovalService tạo mới DraftApprovalService
func NewDraftApprovalService() (*DraftApprovalService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftApprovals)
	if !exist {
		return nil, fmt.Errorf("failed to get content_draft_approvals collection: %v", common.ErrNotFound)
	}

	return &DraftApprovalService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DraftApproval](collection),
	}, nil
}
