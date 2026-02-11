package contentsvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	contentmodels "meta_commerce/internal/api/content/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
	basesvc "meta_commerce/internal/api/base/service"
)

// DraftContentNodeService là service quản lý draft content nodes (L1-L6)
type DraftContentNodeService struct {
	*basesvc.BaseServiceMongoImpl[contentmodels.DraftContentNode]
	contentNodeService *ContentNodeService
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
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[contentmodels.DraftContentNode](collection),
		contentNodeService:   contentNodeService,
	}, nil
}

// InsertOne override để thêm validation sequential level constraint trước khi insert
func (s *DraftContentNodeService) InsertOne(ctx context.Context, data contentmodels.DraftContentNode) (contentmodels.DraftContentNode, error) {
	var parentType string
	var parentExists bool
	var parentIsProduction bool
	var parentIsApproved bool

	if data.ParentID != nil {
		parentProduction, err := s.contentNodeService.FindOneById(ctx, *data.ParentID)
		if err == nil {
			parentType = parentProduction.Type
			parentExists = true
			parentIsProduction = true
			parentIsApproved = true
		} else if err == common.ErrNotFound {
			parentDraft, err := s.FindOneById(ctx, *data.ParentID)
			if err == nil {
				parentType = parentDraft.Type
				parentExists = true
				parentIsProduction = false
				parentIsApproved = (parentDraft.ApprovalStatus == contentmodels.DraftApprovalStatusApproved)
			} else if err == common.ErrNotFound {
				parentExists = false
			} else {
				return data, err
			}
		} else {
			return data, err
		}
	} else if data.ParentDraftID != nil {
		parentDraft, err := s.FindOneById(ctx, *data.ParentDraftID)
		if err == nil {
			parentType = parentDraft.Type
			parentExists = true
			parentIsProduction = false
			parentIsApproved = (parentDraft.ApprovalStatus == contentmodels.DraftApprovalStatusApproved)
		} else if err == common.ErrNotFound {
			parentExists = false
		} else {
			return data, err
		}
	}

	if err := utility.ValidateSequentialLevelConstraint(
		data.Type, parentType, parentExists, parentIsProduction, parentIsApproved,
	); err != nil {
		return data, err
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// UpdateById override để validate approvalStatus
func (s *DraftContentNodeService) UpdateById(ctx context.Context, id primitive.ObjectID, data interface{}) (contentmodels.DraftContentNode, error) {
	updateData, err := basesvc.ToUpdateData(data)
	if err != nil {
		var zero contentmodels.DraftContentNode
		return zero, err
	}
	if updateData.Set != nil {
		if approvalStatus, exists := updateData.Set["approvalStatus"]; exists {
			approvalStatusStr, ok := approvalStatus.(string)
			if !ok {
				var zero contentmodels.DraftContentNode
				return zero, common.NewError(common.ErrCodeValidationFormat, "approvalStatus phải là string", common.StatusBadRequest, nil)
			}
			currentDraft, err := s.FindOneById(ctx, id)
			if err != nil {
				var zero contentmodels.DraftContentNode
				return zero, err
			}
			if approvalStatusStr == contentmodels.DraftApprovalStatusPending {
				if currentDraft.ApprovalStatus != contentmodels.DraftApprovalStatusDraft && currentDraft.ApprovalStatus != contentmodels.DraftApprovalStatusRejected {
					var zero contentmodels.DraftContentNode
					return zero, common.NewError(common.ErrCodeBusinessOperation,
						fmt.Sprintf("Chỉ có thể chuyển status sang pending từ draft hoặc rejected (hiện tại: %s). Để approve/reject, dùng endpoint /drafts/nodes/:id/approve hoặc /reject", currentDraft.ApprovalStatus),
						common.StatusBadRequest, nil)
				}
			} else {
				if approvalStatusStr == contentmodels.DraftApprovalStatusApproved || approvalStatusStr == contentmodels.DraftApprovalStatusRejected {
					var zero contentmodels.DraftContentNode
					return zero, common.NewError(common.ErrCodeBusinessOperation,
						"Không thể update approvalStatus = approved hoặc rejected qua CRUD. Dùng endpoint /drafts/nodes/:id/approve hoặc /reject",
						common.StatusBadRequest, nil)
				}
				if approvalStatusStr == contentmodels.DraftApprovalStatusDraft {
					if currentDraft.ApprovalStatus != contentmodels.DraftApprovalStatusRejected && currentDraft.ApprovalStatus != contentmodels.DraftApprovalStatusApproved {
						var zero contentmodels.DraftContentNode
						return zero, common.NewError(common.ErrCodeBusinessOperation,
							fmt.Sprintf("Chỉ có thể chuyển status về draft từ rejected hoặc approved (hiện tại: %s)", currentDraft.ApprovalStatus),
							common.StatusBadRequest, nil)
					}
				}
			}
		}
	}
	return s.BaseServiceMongoImpl.UpdateById(ctx, id, data)
}

// CommitDraftNode commit draft node → production content node
func (s *DraftContentNodeService) CommitDraftNode(ctx context.Context, draftID primitive.ObjectID) (*contentmodels.ContentNode, error) {
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if draft.ApprovalStatus != contentmodels.DraftApprovalStatusApproved {
		return nil, common.NewError(common.ErrCodeBusinessOperation, "Chỉ có thể commit draft đã được approve", common.StatusBadRequest, nil)
	}
	var parentType string
	var parentExists bool
	var parentIsProduction bool
	if draft.ParentID != nil {
		parentProduction, err := s.contentNodeService.FindOneById(ctx, *draft.ParentID)
		if err == nil {
			parentType = parentProduction.Type
			parentExists = true
			parentIsProduction = true
		} else if err == common.ErrNotFound {
			return nil, common.NewError(common.ErrCodeBusinessOperation,
				fmt.Sprintf("Parent node (ID: %s) không tồn tại trong production. Phải commit parent trước khi commit %s (L%d)",
					draft.ParentID.Hex(), draft.Type, utility.GetContentLevel(draft.Type)), common.StatusBadRequest, nil)
		} else {
			return nil, err
		}
	} else if draft.ParentDraftID != nil {
		return nil, common.NewError(common.ErrCodeBusinessOperation,
			fmt.Sprintf("Parent node (draft ID: %s) chưa được commit. Phải commit parent trước khi commit %s (L%d)",
				draft.ParentDraftID.Hex(), draft.Type, utility.GetContentLevel(draft.Type)), common.StatusBadRequest, nil)
	}
	if err := utility.ValidateSequentialLevelConstraint(draft.Type, parentType, parentExists, parentIsProduction, true); err != nil {
		return nil, err
	}
	contentNode := contentmodels.ContentNode{
		Type:                 draft.Type,
		ParentID:             draft.ParentID,
		Name:                 draft.Name,
		Text:                 draft.Text,
		CreatorType:          contentmodels.CreatorTypeAI,
		CreationMethod:       contentmodels.CreationMethodWorkflow,
		CreatedByRunID:       draft.WorkflowRunID,
		CreatedByStepRunID:   draft.CreatedByStepRunID,
		CreatedByCandidateID: draft.CreatedByCandidateID,
		CreatedByBatchID:     draft.CreatedByBatchID,
		Status:               "active",
		OwnerOrganizationID:  draft.OwnerOrganizationID,
		Metadata:             draft.Metadata,
	}
	result, err := s.contentNodeService.InsertOne(ctx, contentNode)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDraftsByWorkflowRunID lấy tất cả draft nodes của một workflow run
func (s *DraftContentNodeService) GetDraftsByWorkflowRunID(ctx context.Context, workflowRunID primitive.ObjectID) ([]contentmodels.DraftContentNode, error) {
	filter := bson.M{"workflowRunId": workflowRunID}
	return s.Find(ctx, filter, nil)
}

// ApproveDraft duyệt một draft: set approvalStatus = approved
func (s *DraftContentNodeService) ApproveDraft(ctx context.Context, draftID primitive.ObjectID) (*contentmodels.DraftContentNode, error) {
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if draft.ApprovalStatus != contentmodels.DraftApprovalStatusPending && draft.ApprovalStatus != contentmodels.DraftApprovalStatusDraft {
		return nil, common.NewError(common.ErrCodeBusinessOperation,
			fmt.Sprintf("Chỉ có thể approve draft có status = pending hoặc draft (hiện tại: %s)", draft.ApprovalStatus), common.StatusBadRequest, nil)
	}
	updated, err := s.UpdateById(ctx, draftID, &basesvc.UpdateData{
		Set: map[string]interface{}{"approvalStatus": contentmodels.DraftApprovalStatusApproved},
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// RejectDraft từ chối một draft: set approvalStatus = rejected
func (s *DraftContentNodeService) RejectDraft(ctx context.Context, draftID primitive.ObjectID) (*contentmodels.DraftContentNode, error) {
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if draft.ApprovalStatus != contentmodels.DraftApprovalStatusPending && draft.ApprovalStatus != contentmodels.DraftApprovalStatusDraft {
		return nil, common.NewError(common.ErrCodeBusinessOperation,
			fmt.Sprintf("Chỉ có thể reject draft có status = pending hoặc draft (hiện tại: %s)", draft.ApprovalStatus), common.StatusBadRequest, nil)
	}
	updated, err := s.UpdateById(ctx, draftID, &basesvc.UpdateData{
		Set: map[string]interface{}{"approvalStatus": contentmodels.DraftApprovalStatusRejected},
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
