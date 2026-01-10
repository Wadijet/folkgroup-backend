package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftContentNodeService là service quản lý draft content nodes (L1-L6)
type DraftContentNodeService struct {
	*BaseServiceMongoImpl[models.DraftContentNode]
	contentNodeService *ContentNodeService // Service để commit draft → production
}

// NewDraftContentNodeService tạo mới DraftContentNodeService
func NewDraftContentNodeService() (*DraftContentNodeService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DraftContentNodes)
	if !exist {
		return nil, fmt.Errorf("failed to get draft_content_nodes collection: %v", common.ErrNotFound)
	}

	contentNodeService, err := NewContentNodeService()
	if err != nil {
		return nil, fmt.Errorf("failed to create content node service: %v", err)
	}

	return &DraftContentNodeService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DraftContentNode](collection),
		contentNodeService:   contentNodeService,
	}, nil
}

// CommitDraftNode commit draft node → production content node
// Tham số:
//   - ctx: Context
//   - draftID: ID của draft node cần commit
// Trả về:
//   - models.ContentNode: Content node đã được tạo từ draft
//   - error: Lỗi nếu có
func (s *DraftContentNodeService) CommitDraftNode(ctx context.Context, draftID primitive.ObjectID) (*models.ContentNode, error) {
	// Lấy draft node
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}

	// Kiểm tra approval status
	if draft.ApprovalStatus != models.DraftApprovalStatusApproved {
		return nil, common.NewError(
			common.ErrCodeBusinessOperation,
			"Chỉ có thể commit draft đã được approve",
			common.StatusBadRequest,
			nil,
		)
	}

	// Tạo content node từ draft
	contentNode := models.ContentNode{
		Type:                draft.Type,
		ParentID:            draft.ParentID,
		Name:                draft.Name,
		Text:                draft.Text,
		CreatorType:         models.CreatorTypeAI, // Mặc định là AI vì draft thường từ workflow
		CreationMethod:      models.CreationMethodWorkflow,
		CreatedByRunID:      draft.WorkflowRunID,
		CreatedByStepRunID:  draft.CreatedByStepRunID,
		CreatedByCandidateID: draft.CreatedByCandidateID,
		CreatedByBatchID:    draft.CreatedByBatchID,
		Status:              "active",
		OwnerOrganizationID: draft.OwnerOrganizationID,
		Metadata:            draft.Metadata,
	}

	// Insert vào production
	result, err := s.contentNodeService.InsertOne(ctx, contentNode)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDraftsByWorkflowRunID lấy tất cả draft nodes của một workflow run
// Tham số:
//   - ctx: Context
//   - workflowRunID: ID của workflow run
// Trả về:
//   - []models.DraftContentNode: Danh sách draft nodes
//   - error: Lỗi nếu có
func (s *DraftContentNodeService) GetDraftsByWorkflowRunID(ctx context.Context, workflowRunID primitive.ObjectID) ([]models.DraftContentNode, error) {
	filter := bson.M{
		"workflowRunId": workflowRunID,
	}
	return s.Find(ctx, filter, nil)
}
