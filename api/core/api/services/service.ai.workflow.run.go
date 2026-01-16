package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
	"meta_commerce/core/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowRunService là service quản lý AI workflow runs (Module 2)
type AIWorkflowRunService struct {
	*BaseServiceMongoImpl[models.AIWorkflowRun]
}

// NewAIWorkflowRunService tạo mới AIWorkflowRunService
// Trả về:
//   - *AIWorkflowRunService: Instance mới của AIWorkflowRunService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowRunService() (*AIWorkflowRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_runs collection: %v", common.ErrNotFound)
	}

	return &AIWorkflowRunService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIWorkflowRun](collection),
	}, nil
}

// ValidateRootRef validate RootRefID và RootRefType (business logic validation)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Cross-collection validation:
//    - Validate RootRefID tồn tại trong production (ContentNode) hoặc draft (DraftContentNode)
//    - Kiểm tra RootRefType đúng với type của RootRefID
//    - Kiểm tra RootRefID đã được commit (production) hoặc là draft đã được approve
//    - Đảm bảo workflow chỉ bắt đầu từ content đã sẵn sàng
//
// 2. Business rules:
//    - RootRefType phải hợp lệ (layer, stp, insight, contentLine, gene, script)
//    - RootRefID phải tồn tại và đúng type
//    - Draft phải đã được approve trước khi bắt đầu workflow
//
// Tham số:
//   - ctx: Context
//   - rootRefID: ID của root content (có thể nil nếu không có)
//   - rootRefType: Loại root reference (có thể rỗng nếu không có)
//
// Trả về:
//   - error: Lỗi nếu validation thất bại, nil nếu hợp lệ
func (s *AIWorkflowRunService) ValidateRootRef(ctx context.Context, rootRefID *primitive.ObjectID, rootRefType string) error {
	// Nếu không có RootRefID hoặc RootRefType, không cần validate
	if rootRefID == nil || rootRefType == "" {
		return nil
	}

	// Lấy content node service và draft content node service để kiểm tra
	contentNodeService, err := NewContentNodeService()
	if err != nil {
		return fmt.Errorf("lỗi khi khởi tạo content node service: %v", err)
	}

	draftContentNodeService, err := NewDraftContentNodeService()
	if err != nil {
		return fmt.Errorf("lỗi khi khởi tạo draft content node service: %v", err)
	}

	// Kiểm tra rootRefID tồn tại và đúng type
	var rootType string
	var rootExists bool
	var rootIsProduction bool
	var rootIsApproved bool

	// Thử tìm trong production trước
	rootProduction, err := contentNodeService.FindOneById(ctx, *rootRefID)
	if err == nil {
		// Root tồn tại trong production
		rootType = rootProduction.Type
		rootExists = true
		rootIsProduction = true
		rootIsApproved = true // Production = đã approve
	} else if err == common.ErrNotFound {
		// Không tìm thấy trong production, thử tìm trong draft
		rootDraft, err := draftContentNodeService.FindOneById(ctx, *rootRefID)
		if err == nil {
			// Root tồn tại trong draft
			rootType = rootDraft.Type
			rootExists = true
			rootIsProduction = false
			rootIsApproved = (rootDraft.ApprovalStatus == models.DraftApprovalStatusApproved)
		} else if err == common.ErrNotFound {
			// Root không tồn tại
			rootExists = false
		} else {
			return fmt.Errorf("lỗi khi tìm root draft: %v", err)
		}
	} else {
		return fmt.Errorf("lỗi khi tìm root production: %v", err)
	}

	// Kiểm tra rootRefID tồn tại
	if !rootExists {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("RootRefID '%s' không tồn tại trong production hoặc draft", rootRefID.Hex()),
			common.StatusBadRequest,
			nil,
		)
	}

	// Kiểm tra RootRefType đúng với type của rootRefID
	if rootType != rootRefType {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("RootRefType '%s' không khớp với type của RootRefID. RootRefID có type: '%s'", rootRefType, rootType),
			common.StatusBadRequest,
			nil,
		)
	}

	// Kiểm tra rootRefID đã được commit (production) hoặc là draft đã được approve
	// Đây là validation để đảm bảo workflow chỉ bắt đầu từ content đã sẵn sàng
	if !rootIsProduction {
		// Root là draft, phải đã được approve
		if !rootIsApproved {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("RootRefID '%s' (type: %s) là draft chưa được approve. Phải approve và commit root trước khi bắt đầu workflow",
					rootRefID.Hex(), rootType),
				common.StatusBadRequest,
				nil,
			)
		}
	}

	// Validate sequential level constraint: RootRefType phải hợp lệ
	rootLevel := utility.GetContentLevel(rootType)
	if rootLevel == 0 {
		return common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("RootRefType '%s' không hợp lệ. Các type hợp lệ: layer, stp, insight, contentLine, gene, script", rootType),
			common.StatusBadRequest,
			nil,
		)
	}

	return nil
}

// InsertOne override để thêm business logic validation trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation:
//    - Validate RootRefID và RootRefType (cross-collection validation)
//    - Đảm bảo workflow chỉ bắt đầu từ content đã sẵn sàng
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate RootRefID và RootRefType bằng ValidateRootRef()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *AIWorkflowRunService) InsertOne(ctx context.Context, data models.AIWorkflowRun) (models.AIWorkflowRun, error) {
	// Validate RootRefID và RootRefType (business logic validation)
	if err := s.ValidateRootRef(ctx, data.RootRefID, data.RootRefType); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
