package aisvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	aimodels "meta_commerce/internal/api/ai/models"
	contentmodels "meta_commerce/internal/api/content/models"
	contentsvc "meta_commerce/internal/api/content/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIWorkflowRunService là service quản lý AI workflow runs (Module 2)
type AIWorkflowRunService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIWorkflowRun]
}

// NewAIWorkflowRunService tạo mới AIWorkflowRunService
func NewAIWorkflowRunService() (*AIWorkflowRunService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowRuns)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_runs collection: %v", common.ErrNotFound)
	}
	return &AIWorkflowRunService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIWorkflowRun](collection),
	}, nil
}

// ValidateRootRef validate RootRefID và RootRefType (cross-collection với content/draft)
func (s *AIWorkflowRunService) ValidateRootRef(ctx context.Context, rootRefID *primitive.ObjectID, rootRefType string) error {
	if rootRefID == nil || rootRefType == "" {
		return nil
	}
	contentNodeService, err := contentsvc.NewContentNodeService()
	if err != nil {
		return fmt.Errorf("lỗi khi khởi tạo content node service: %v", err)
	}
	draftContentNodeService, err := contentsvc.NewDraftContentNodeService()
	if err != nil {
		return fmt.Errorf("lỗi khi khởi tạo draft content node service: %v", err)
	}
	var rootType string
	var rootExists bool
	var rootIsProduction bool
	var rootIsApproved bool
	rootProduction, err := contentNodeService.FindOneById(ctx, *rootRefID)
	if err == nil {
		rootType = rootProduction.Type
		rootExists = true
		rootIsProduction = true
		rootIsApproved = true
	} else if err == common.ErrNotFound {
		rootDraft, err := draftContentNodeService.FindOneById(ctx, *rootRefID)
		if err == nil {
			rootType = rootDraft.Type
			rootExists = true
			rootIsProduction = false
			rootIsApproved = (rootDraft.ApprovalStatus == contentmodels.DraftApprovalStatusApproved)
		} else if err == common.ErrNotFound {
			rootExists = false
		} else {
			return fmt.Errorf("lỗi khi tìm root draft: %v", err)
		}
	} else {
		return fmt.Errorf("lỗi khi tìm root production: %v", err)
	}
	if !rootExists {
		return common.NewError(common.ErrCodeBusinessOperation,
			fmt.Sprintf("RootRefID '%s' không tồn tại trong production hoặc draft", rootRefID.Hex()), common.StatusBadRequest, nil)
	}
	if rootType != rootRefType {
		return common.NewError(common.ErrCodeBusinessOperation,
			fmt.Sprintf("RootRefType '%s' không khớp với type của RootRefID. RootRefID có type: '%s'", rootRefType, rootType), common.StatusBadRequest, nil)
	}
	if !rootIsProduction {
		if !rootIsApproved {
			return common.NewError(common.ErrCodeBusinessOperation,
				fmt.Sprintf("RootRefID '%s' (type: %s) là draft chưa được approve. Phải approve và commit root trước khi bắt đầu workflow", rootRefID.Hex(), rootType),
				common.StatusBadRequest, nil)
		}
	}
	rootLevel := utility.GetContentLevel(rootType)
	if rootLevel == 0 {
		return common.NewError(common.ErrCodeValidationFormat,
			fmt.Sprintf("RootRefType '%s' không hợp lệ. Các type hợp lệ: pillar, stp, insight, contentLine, gene, script", rootType), common.StatusBadRequest, nil)
	}
	return nil
}

// InsertOne override để validate RootRef trước khi insert
func (s *AIWorkflowRunService) InsertOne(ctx context.Context, data aimodels.AIWorkflowRun) (aimodels.AIWorkflowRun, error) {
	if err := s.ValidateRootRef(ctx, data.RootRefID, data.RootRefType); err != nil {
		return data, err
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
