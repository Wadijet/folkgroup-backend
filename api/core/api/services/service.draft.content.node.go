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

// InsertOne override để thêm validation sequential level constraint trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation - Sequential level constraint:
//    - Kiểm tra parent phải tồn tại trong production (ContentNode) hoặc draft (DraftContentNode)
//    - Kiểm tra parent đã được commit (production) hoặc là draft đã được approve
//    - Validate sequential level constraint: parent phải có level thấp hơn child đúng 1 level
//    - Đảm bảo cấu trúc content hierarchy hợp lệ (L1 → L2 → L3 → ... → L6)
//
// 2. Cross-collection validation:
//    - Query parent từ ContentNode collection (production)
//    - Query parent từ DraftContentNode collection (draft)
//    - Xử lý cả ParentID và ParentDraftID
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate sequential level constraint bằng ValidateSequentialLevelConstraint()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
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

// UpdateById override để validate approvalStatus: không cho phép update approvalStatus trực tiếp qua CRUD.
// Chỉ cho phép update approvalStatus qua endpoint approve/reject riêng (có validation đầy đủ).
//
// LÝ DO:
//   - Bảo vệ luồng approval: không cho user/bot set approvalStatus tùy ý
//   - Chỉ cho phép chuyển draft → pending (user có thể gửi duyệt)
//   - Các chuyển đổi khác (pending → approved, pending → rejected, etc.) phải qua endpoint riêng
func (s *DraftContentNodeService) UpdateById(ctx context.Context, id primitive.ObjectID, data interface{}) (models.DraftContentNode, error) {
	// Convert data thành UpdateData
	updateData, err := ToUpdateData(data)
	if err != nil {
		var zero models.DraftContentNode
		return zero, err
	}

	// Kiểm tra nếu có update approvalStatus
	if updateData.Set != nil {
		if approvalStatus, exists := updateData.Set["approvalStatus"]; exists {
			approvalStatusStr, ok := approvalStatus.(string)
			if !ok {
				var zero models.DraftContentNode
				return zero, common.NewError(
					common.ErrCodeValidationFormat,
					"approvalStatus phải là string",
					common.StatusBadRequest,
					nil,
				)
			}

			// Lấy draft hiện tại để kiểm tra status
			currentDraft, err := s.FindOneById(ctx, id)
			if err != nil {
				var zero models.DraftContentNode
				return zero, err
			}

			// Chỉ cho phép chuyển draft → pending (user gửi duyệt)
			// Các chuyển đổi khác phải qua endpoint approve/reject
			if approvalStatusStr == models.DraftApprovalStatusPending {
				if currentDraft.ApprovalStatus != models.DraftApprovalStatusDraft && currentDraft.ApprovalStatus != models.DraftApprovalStatusRejected {
					var zero models.DraftContentNode
					return zero, common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("Chỉ có thể chuyển status sang pending từ draft hoặc rejected (hiện tại: %s). Để approve/reject, dùng endpoint /drafts/nodes/:id/approve hoặc /reject", currentDraft.ApprovalStatus),
						common.StatusBadRequest,
						nil,
					)
				}
			} else {
				// Không cho phép set approvalStatus = approved hoặc rejected qua CRUD
				if approvalStatusStr == models.DraftApprovalStatusApproved || approvalStatusStr == models.DraftApprovalStatusRejected {
					var zero models.DraftContentNode
					return zero, common.NewError(
						common.ErrCodeBusinessOperation,
						"Không thể update approvalStatus = approved hoặc rejected qua CRUD. Dùng endpoint /drafts/nodes/:id/approve hoặc /reject",
						common.StatusBadRequest,
						nil,
					)
				}
				// Cho phép set về draft (chỉnh sửa lại)
				if approvalStatusStr == models.DraftApprovalStatusDraft {
					if currentDraft.ApprovalStatus != models.DraftApprovalStatusRejected && currentDraft.ApprovalStatus != models.DraftApprovalStatusApproved {
						var zero models.DraftContentNode
						return zero, common.NewError(
							common.ErrCodeBusinessOperation,
							fmt.Sprintf("Chỉ có thể chuyển status về draft từ rejected hoặc approved (hiện tại: %s)", currentDraft.ApprovalStatus),
							common.StatusBadRequest,
							nil,
						)
					}
				}
			}
		}
	}

	// Gọi UpdateById của base service
	return s.BaseServiceMongoImpl.UpdateById(ctx, id, data)
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

// ApproveDraft duyệt một draft: set approvalStatus = approved (chỉ khi status = pending hoặc draft).
// Không tự động commit, user phải gọi CommitDraftNode riêng.
//
// Tham số:
//   - ctx: Context
//   - draftID: ID của draft cần approve
//
// Trả về:
//   - *models.DraftContentNode: Draft đã được update
//   - error: Lỗi nếu có (status không hợp lệ, draft không tồn tại, etc.)
func (s *DraftContentNodeService) ApproveDraft(ctx context.Context, draftID primitive.ObjectID) (*models.DraftContentNode, error) {
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}

	// Validate: chỉ approve khi status = pending hoặc draft
	if draft.ApprovalStatus != models.DraftApprovalStatusPending && draft.ApprovalStatus != models.DraftApprovalStatusDraft {
		return nil, common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Chỉ có thể approve draft có status = pending hoặc draft (hiện tại: %s)", draft.ApprovalStatus),
			common.StatusBadRequest,
			nil,
		)
	}

	// Update status
	updated, err := s.UpdateById(ctx, draftID, &UpdateData{
		Set: map[string]interface{}{"approvalStatus": models.DraftApprovalStatusApproved},
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

// RejectDraft từ chối một draft: set approvalStatus = rejected (chỉ khi status = pending hoặc draft).
//
// Tham số:
//   - ctx: Context
//   - draftID: ID của draft cần reject
//
// Trả về:
//   - *models.DraftContentNode: Draft đã được update
//   - error: Lỗi nếu có (status không hợp lệ, draft không tồn tại, etc.)
func (s *DraftContentNodeService) RejectDraft(ctx context.Context, draftID primitive.ObjectID) (*models.DraftContentNode, error) {
	draft, err := s.FindOneById(ctx, draftID)
	if err != nil {
		return nil, err
	}

	// Validate: chỉ reject khi status = pending hoặc draft
	if draft.ApprovalStatus != models.DraftApprovalStatusPending && draft.ApprovalStatus != models.DraftApprovalStatusDraft {
		return nil, common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Chỉ có thể reject draft có status = pending hoặc draft (hiện tại: %s)", draft.ApprovalStatus),
			common.StatusBadRequest,
			nil,
		)
	}

	// Update status
	updated, err := s.UpdateById(ctx, draftID, &UpdateData{
		Set: map[string]interface{}{"approvalStatus": models.DraftApprovalStatusRejected},
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}
