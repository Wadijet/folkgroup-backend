package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
	"meta_commerce/core/utility"

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

// InsertOne override để thêm validation sequential level constraint
// Kiểm tra parent phải tồn tại và đã được commit (production) hoặc là draft đã được approve
func (s *DraftContentNodeService) InsertOne(ctx context.Context, data models.DraftContentNode) (models.DraftContentNode, error) {
	// Validate sequential level constraint
	var parentType string
	var parentExists bool
	var parentIsProduction bool
	var parentIsApproved bool

	// Nếu có parent, kiểm tra parent
	if data.ParentID != nil {
		// Thử tìm parent trong production trước
		parentProduction, err := s.contentNodeService.FindOneById(ctx, *data.ParentID)
		if err == nil {
			// Parent tồn tại trong production
			parentType = parentProduction.Type
			parentExists = true
			parentIsProduction = true
			parentIsApproved = true // Production = đã approve
		} else if err == common.ErrNotFound {
			// Không tìm thấy trong production, thử tìm trong draft
			parentDraft, err := s.FindOneById(ctx, *data.ParentID)
			if err == nil {
				// Parent tồn tại trong draft
				parentType = parentDraft.Type
				parentExists = true
				parentIsProduction = false
				parentIsApproved = (parentDraft.ApprovalStatus == models.DraftApprovalStatusApproved)
			} else if err == common.ErrNotFound {
				// Parent không tồn tại
				parentExists = false
			} else {
				return data, err
			}
		} else {
			return data, err
		}
	} else if data.ParentDraftID != nil {
		// Nếu có ParentDraftID, kiểm tra draft parent
		parentDraft, err := s.FindOneById(ctx, *data.ParentDraftID)
		if err == nil {
			parentType = parentDraft.Type
			parentExists = true
			parentIsProduction = false
			parentIsApproved = (parentDraft.ApprovalStatus == models.DraftApprovalStatusApproved)
		} else if err == common.ErrNotFound {
			parentExists = false
		} else {
			return data, err
		}
	}

	// Validate sequential level constraint
	if err := utility.ValidateSequentialLevelConstraint(
		data.Type,
		parentType,
		parentExists,
		parentIsProduction,
		parentIsApproved,
	); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// CommitDraftNode commit draft node → production content node
// Tham số:
//   - ctx: Context
//   - draftID: ID của draft node cần commit
//
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

	// Validate sequential level constraint: parent phải đã được commit (production)
	var parentType string
	var parentExists bool
	var parentIsProduction bool

	if draft.ParentID != nil {
		// Kiểm tra parent trong production
		parentProduction, err := s.contentNodeService.FindOneById(ctx, *draft.ParentID)
		if err == nil {
			parentType = parentProduction.Type
			parentExists = true
			parentIsProduction = true
		} else if err == common.ErrNotFound {
			// Parent không tồn tại trong production
			return nil, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Parent node (ID: %s) không tồn tại trong production. Phải commit parent trước khi commit %s (L%d)",
					draft.ParentID.Hex(), draft.Type, utility.GetContentLevel(draft.Type)),
				common.StatusBadRequest,
				nil,
			)
		} else {
			return nil, err
		}
	} else if draft.ParentDraftID != nil {
		// Nếu parent là draft, không thể commit (parent phải là production)
		return nil, common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Parent node (draft ID: %s) chưa được commit. Phải commit parent trước khi commit %s (L%d)",
				draft.ParentDraftID.Hex(), draft.Type, utility.GetContentLevel(draft.Type)),
			common.StatusBadRequest,
			nil,
		)
	}

	// Validate sequential level constraint
	if err := utility.ValidateSequentialLevelConstraint(
		draft.Type,
		parentType,
		parentExists,
		parentIsProduction,
		true, // Parent production = đã approve
	); err != nil {
		return nil, err
	}

	// Tạo content node từ draft
	contentNode := models.ContentNode{
		Type:                 draft.Type,
		ParentID:             draft.ParentID,
		Name:                 draft.Name,
		Text:                 draft.Text,
		CreatorType:          models.CreatorTypeAI, // Mặc định là AI vì draft thường từ workflow
		CreationMethod:       models.CreationMethodWorkflow,
		CreatedByRunID:       draft.WorkflowRunID,
		CreatedByStepRunID:   draft.CreatedByStepRunID,
		CreatedByCandidateID: draft.CreatedByCandidateID,
		CreatedByBatchID:     draft.CreatedByBatchID,
		Status:               "active",
		OwnerOrganizationID:  draft.OwnerOrganizationID,
		Metadata:             draft.Metadata,
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
//
// Trả về:
//   - []models.DraftContentNode: Danh sách draft nodes
//   - error: Lỗi nếu có
func (s *DraftContentNodeService) GetDraftsByWorkflowRunID(ctx context.Context, workflowRunID primitive.ObjectID) ([]models.DraftContentNode, error) {
	filter := bson.M{
		"workflowRunId": workflowRunID,
	}
	return s.Find(ctx, filter, nil)
}
