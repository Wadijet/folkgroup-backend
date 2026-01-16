package services

import (
	"context"
	"fmt"
	"time"

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

// ValidateTargets validate cross-field: phải có ít nhất một target (business logic validation)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Cross-field validation:
//    - Phải có ít nhất một target: workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID
//    - Đây là validation cross-field (kiểm tra nhiều field cùng lúc), không thể dùng struct tag đơn giản
//
// Tham số:
//   - approval: Draft approval cần validate
//
// Trả về:
//   - error: Lỗi nếu validation thất bại, nil nếu hợp lệ
func (s *DraftApprovalService) ValidateTargets(approval models.DraftApproval) error {
	// Validate: Phải có ít nhất một target
	hasTarget := approval.WorkflowRunID != nil && !approval.WorkflowRunID.IsZero() ||
		approval.DraftNodeID != nil && !approval.DraftNodeID.IsZero() ||
		approval.DraftVideoID != nil && !approval.DraftVideoID.IsZero() ||
		approval.DraftPublicationID != nil && !approval.DraftPublicationID.IsZero()

	if !hasTarget {
		return common.NewError(
			common.ErrCodeValidationFormat,
			"Phải có ít nhất một target: workflowRunId, draftNodeId, draftVideoId, hoặc draftPublicationId",
			common.StatusBadRequest,
			nil,
		)
	}

	return nil
}

// PrepareForInsert chuẩn bị approval model trước khi insert (business logic)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Set fields tự động:
//    - Set RequestedBy từ userID (từ context)
//    - Set RequestedAt tự động (timestamp hiện tại)
//    - Set Status = "pending" mặc định (không cho phép client chỉ định status khi tạo)
//
// Tham số:
//   - ctx: Context (để lấy userID)
//   - approval: Draft approval cần chuẩn bị
//
// Trả về:
//   - error: Lỗi nếu không lấy được userID, nil nếu thành công
func (s *DraftApprovalService) PrepareForInsert(ctx context.Context, approval *models.DraftApproval) error {
	// Lấy userID từ context
	userID, ok := GetUserIDFromContext(ctx)
	if !ok || userID.IsZero() {
		return common.NewError(
			common.ErrCodeAuthToken,
			"Không tìm thấy user ID trong context",
			common.StatusUnauthorized,
			nil,
		)
	}

	// Set RequestedBy, RequestedAt, Status
	approval.RequestedBy = userID
	approval.RequestedAt = time.Now().UnixMilli()
	approval.Status = models.ApprovalRequestStatusPending

	return nil
}

// InsertOne override để thêm business logic validation và prepare trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation:
//    - Validate cross-field: phải có ít nhất một target
//    - Prepare model: set RequestedBy, RequestedAt, Status
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate targets bằng ValidateTargets()
// ✅ Prepare model bằng PrepareForInsert()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *DraftApprovalService) InsertOne(ctx context.Context, data models.DraftApproval) (models.DraftApproval, error) {
	// Validate targets (business logic validation)
	if err := s.ValidateTargets(data); err != nil {
		return data, err
	}

	// Prepare model (set RequestedBy, RequestedAt, Status)
	if err := s.PrepareForInsert(ctx, &data); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
