package aisvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	aimodels "meta_commerce/internal/api/ai/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AIWorkflowCommandService là service quản lý AI workflow commands (Module 2)
type AIWorkflowCommandService struct {
	*basesvc.BaseServiceMongoImpl[aimodels.AIWorkflowCommand]
}

// NewAIWorkflowCommandService tạo mới AIWorkflowCommandService
func NewAIWorkflowCommandService() (*AIWorkflowCommandService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowCommands)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_commands collection: %v", common.ErrNotFound)
	}
	return &AIWorkflowCommandService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[aimodels.AIWorkflowCommand](collection),
	}, nil
}

func isStepCreatePillar(step *aimodels.AIStep) bool {
	if step == nil {
		return false
	}
	return step.ParentLevel == "" && step.TargetLevel == "L1"
}

func (s *AIWorkflowCommandService) resolveStepForCommand(ctx context.Context, data aimodels.AIWorkflowCommand) (*aimodels.AIStep, error) {
	stepService, err := NewAIStepService()
	if err != nil {
		return nil, fmt.Errorf("lỗi khi khởi tạo step service: %w", err)
	}
	if data.CommandType == aimodels.AIWorkflowCommandTypeExecuteStep {
		if data.StepID == nil || data.StepID.IsZero() {
			return nil, common.NewError(common.ErrCodeValidationFormat, "StepID bắt buộc khi CommandType = EXECUTE_STEP", common.StatusBadRequest, nil)
		}
		step, err := stepService.FindOneById(ctx, *data.StepID)
		if err != nil {
			return nil, fmt.Errorf("không tìm thấy step: %w", err)
		}
		return &step, nil
	}
	if data.WorkflowID == nil || data.WorkflowID.IsZero() {
		return nil, common.NewError(common.ErrCodeValidationFormat, "WorkflowID bắt buộc khi CommandType = START_WORKFLOW", common.StatusBadRequest, nil)
	}
	workflowService, err := NewAIWorkflowService()
	if err != nil {
		return nil, fmt.Errorf("lỗi khi khởi tạo workflow service: %w", err)
	}
	workflow, err := workflowService.FindOneById(ctx, *data.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy workflow: %w", err)
	}
	if len(workflow.Steps) == 0 {
		return nil, common.NewError(common.ErrCodeBusinessOperation, "Workflow không có step nào", common.StatusBadRequest, nil)
	}
	firstStepIDStr := workflow.Steps[0].StepID
	firstStepID, err := primitive.ObjectIDFromHex(firstStepIDStr)
	if err != nil {
		return nil, fmt.Errorf("workflow stepId không hợp lệ: %w", err)
	}
	step, err := stepService.FindOneById(ctx, firstStepID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy step đầu tiên của workflow: %w", err)
	}
	return &step, nil
}

// InsertOne override để validate RootRefID/RootRefType theo step
func (s *AIWorkflowCommandService) InsertOne(ctx context.Context, data aimodels.AIWorkflowCommand) (aimodels.AIWorkflowCommand, error) {
	step, err := s.resolveStepForCommand(ctx, data)
	if err != nil {
		return data, err
	}
	if !isStepCreatePillar(step) {
		if data.RootRefID == nil || data.RootRefID.IsZero() {
			return data, common.NewError(common.ErrCodeValidationFormat,
				"RootRefID và RootRefType bắt buộc khi step cần parent (L2-L6). Chỉ khi tạo Pillar (L1) mới được để trống.", common.StatusBadRequest, nil)
		}
		if data.RootRefType == "" {
			return data, common.NewError(common.ErrCodeValidationFormat,
				"RootRefType bắt buộc khi step cần parent (L2-L6). Ví dụ: pillar, stp, insight.", common.StatusBadRequest, nil)
		}
		runService, err := NewAIWorkflowRunService()
		if err != nil {
			return data, fmt.Errorf("lỗi khi khởi tạo workflow run service: %w", err)
		}
		if err := runService.ValidateRootRef(ctx, data.RootRefID, data.RootRefType); err != nil {
			return data, err
		}
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
func (s *AIWorkflowCommandService) ClaimPendingCommands(ctx context.Context, agentId string, limit int) ([]aimodels.AIWorkflowCommand, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}
	now := time.Now().Unix()
	filter := bson.M{
		"status": aimodels.AIWorkflowCommandStatusPending,
		"$or": []bson.M{
			{"agentId": bson.M{"$exists": false}},
			{"agentId": ""},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status": aimodels.AIWorkflowCommandStatusExecuting,
			"agentId": agentId,
			"assignedAt": now,
			"executedAt": now,
			"lastHeartbeatAt": now,
		},
	}
	opts := options.FindOneAndUpdate().
		SetSort(bson.M{"createdAt": 1}).
		SetReturnDocument(options.After)
	coll := s.Collection()
	claimedCommands := make([]aimodels.AIWorkflowCommand, 0, limit) // Dùng [] thay vì nil để JSON trả về data: [] chứ không phải data: null
	for i := 0; i < limit; i++ {
		var command aimodels.AIWorkflowCommand
		err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&command)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) || err == common.ErrNotFound {
				break
			}
			return nil, fmt.Errorf("failed to claim command: %w", err)
		}
		claimedCommands = append(claimedCommands, command)
	}
	return claimedCommands, nil
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
func (s *AIWorkflowCommandService) UpdateHeartbeat(ctx context.Context, commandID primitive.ObjectID, agentId string, progress map[string]interface{}) (*aimodels.AIWorkflowCommand, error) {
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}
	now := time.Now().Unix()
	filter := bson.M{
		"_id": commandID, "agentId": agentId, "status": aimodels.AIWorkflowCommandStatusExecuting,
	}
	update := bson.M{"$set": bson.M{"lastHeartbeatAt": now}}
	if progress != nil {
		update["$set"].(bson.M)["progress"] = progress
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var command aimodels.AIWorkflowCommand
	err := s.Collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(&command)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err == common.ErrNotFound {
			return nil, fmt.Errorf("command không tồn tại, không thuộc về agent này, hoặc đã completed/failed")
		}
		return nil, fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return &command, nil
}

// ReleaseStuckCommands giải phóng các commands bị stuck (quá lâu không có heartbeat)
func (s *AIWorkflowCommandService) ReleaseStuckCommands(ctx context.Context, timeoutSeconds int64) (int64, error) {
	if timeoutSeconds < 60 {
		timeoutSeconds = 300
	}
	now := time.Now().Unix()
	timeoutThreshold := now - timeoutSeconds
	filter := bson.M{
		"status": aimodels.AIWorkflowCommandStatusExecuting,
		"$or": []bson.M{
			{"lastHeartbeatAt": bson.M{"$exists": true, "$lt": timeoutThreshold}},
			{"lastHeartbeatAt": bson.M{"$exists": false}, "executedAt": bson.M{"$exists": true, "$lt": timeoutThreshold}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status": aimodels.AIWorkflowCommandStatusPending, "agentId": "", "assignedAt": 0,
			"lastHeartbeatAt": 0, "executedAt": 0,
		},
		"$unset": bson.M{"progress": ""},
	}
	result, err := s.Collection().UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to release stuck commands: %w", err)
	}
	return result.ModifiedCount, nil
}
