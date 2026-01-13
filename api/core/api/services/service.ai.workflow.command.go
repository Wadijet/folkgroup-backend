package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AIWorkflowCommandService là service quản lý AI workflow commands (Module 2)
type AIWorkflowCommandService struct {
	*BaseServiceMongoImpl[models.AIWorkflowCommand]
}

// NewAIWorkflowCommandService tạo mới AIWorkflowCommandService
// Trả về:
//   - *AIWorkflowCommandService: Instance mới của AIWorkflowCommandService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIWorkflowCommandService() (*AIWorkflowCommandService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AIWorkflowCommands)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_workflow_commands collection: %v", common.ErrNotFound)
	}

	return &AIWorkflowCommandService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIWorkflowCommand](collection),
	}, nil
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
// Đảm bảo các job khác không lấy lại commands đã được claim cho đến khi được giải phóng
//
// Tham số:
//   - ctx: Context
//   - agentId: ID của agent đang claim commands (string)
//   - limit: Số lượng commands tối đa muốn claim (tối thiểu 1, tối đa 100)
//
// Trả về:
//   - []models.AIWorkflowCommand: Danh sách commands đã được claim (có thể rỗng nếu không có command pending)
//   - Commands được claim sẽ có status = "executing" và agentId được set
//   - error: Lỗi nếu có trong quá trình claim
//
// Logic:
//   1. Tìm các commands có status = "pending" và agentId không tồn tại hoặc rỗng
//   2. Atomic update: Set status = "executing", agentId = agentId, assignedAt = now
//   3. Trả về danh sách commands đã được claim
//
// Lưu ý:
//   - Operation này là atomic, đảm bảo không có race condition
//   - Nếu không có command pending, trả về mảng rỗng (không phải lỗi)
//   - Agent cần giải phóng command bằng cách update status khi hoàn thành hoặc thất bại
func (s *AIWorkflowCommandService) ClaimPendingCommands(ctx context.Context, agentId string, limit int) ([]models.AIWorkflowCommand, error) {
	// Validate limit
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	// Validate agentId
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}

	now := time.Now().Unix()

	// Filter: Tìm commands có status = "pending" và agentId không tồn tại hoặc rỗng
	filter := bson.M{
		"status": models.AIWorkflowCommandStatusPending,
		"$or": []bson.M{
			{"agentId": bson.M{"$exists": false}}, // agentId không tồn tại
			{"agentId": ""},                        // agentId rỗng
		},
	}

	// Update: Set status = "executing", agentId = agentId, assignedAt = now, lastHeartbeatAt = now
	update := bson.M{
		"$set": bson.M{
			"status":          models.AIWorkflowCommandStatusExecuting,
			"agentId":         agentId,
			"assignedAt":      now,
			"executedAt":      now,
			"lastHeartbeatAt": now, // Set heartbeat ngay khi claim
		},
	}

	// Options: Sort theo createdAt tăng dần (FIFO), limit số lượng
	opts := options.FindOneAndUpdate().
		SetSort(bson.M{"createdAt": 1}).
		SetReturnDocument(options.After) // Trả về document sau khi update

	// Claim từng command một cho đến khi đủ limit hoặc không còn command pending
	var claimedCommands []models.AIWorkflowCommand

	for i := 0; i < limit; i++ {
		var command models.AIWorkflowCommand
		err := s.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&command)
		if err != nil {
			// Nếu không tìm thấy command (ErrNoDocuments), dừng lại
			if errors.Is(err, mongo.ErrNoDocuments) || err == common.ErrNotFound {
				break // Không còn command pending, dừng lại
			}
			// Lỗi khác, trả về lỗi
			return nil, fmt.Errorf("failed to claim command: %w", err)
		}

		claimedCommands = append(claimedCommands, command)
	}

	// Trả về danh sách commands đã claim (có thể rỗng nếu không có command pending)
	return claimedCommands, nil
}

// UpdateHeartbeat cập nhật heartbeat và progress của command
// Agent phải gọi method này định kỳ để server biết job đang được thực hiện
//
// Tham số:
//   - ctx: Context
//   - commandID: ID của command cần update
//   - agentId: ID của agent đang xử lý command (để verify ownership)
//   - progress: Tiến độ chi tiết (tùy chọn)
//
// Trả về:
//   - *models.AIWorkflowCommand: Command đã được update
//   - error: Lỗi nếu có (ví dụ: command không tồn tại, không thuộc về agent này, hoặc đã completed/failed)
//
// Lưu ý:
//   - Chỉ update được nếu command có status = "executing" và agentId khớp
//   - Nếu command đã completed/failed, không cho phép update
func (s *AIWorkflowCommandService) UpdateHeartbeat(ctx context.Context, commandID primitive.ObjectID, agentId string, progress map[string]interface{}) (*models.AIWorkflowCommand, error) {
	// Validate agentId
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}

	now := time.Now().Unix()

	// Filter: Tìm command có ID và agentId khớp, status = "executing"
	filter := bson.M{
		"_id":     commandID,
		"agentId": agentId,
		"status":  models.AIWorkflowCommandStatusExecuting,
	}

	// Update: Set lastHeartbeatAt = now và progress (nếu có)
	update := bson.M{
		"$set": bson.M{
			"lastHeartbeatAt": now,
		},
	}

	// Nếu có progress, thêm vào update
	if progress != nil {
		update["$set"].(bson.M)["progress"] = progress
	}

	// Options: Trả về document sau khi update
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After)

	var command models.AIWorkflowCommand
	err := s.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&command)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err == common.ErrNotFound {
			return nil, fmt.Errorf("command không tồn tại, không thuộc về agent này, hoặc đã completed/failed")
		}
		return nil, fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return &command, nil
}

// ReleaseStuckCommands giải phóng các commands bị stuck (quá lâu không có heartbeat)
// Tự động reset status về "pending" và clear agentId để các job khác có thể claim
//
// Tham số:
//   - ctx: Context
//   - timeoutSeconds: Thời gian timeout (giây) - nếu lastHeartbeatAt cũ hơn (now - timeoutSeconds) thì coi là stuck
//                      Mặc định: 300 giây (5 phút)
//
// Trả về:
//   - int64: Số lượng commands đã được giải phóng
//   - error: Lỗi nếu có
//
// Lưu ý:
//   - Chỉ giải phóng commands có status = "executing"
//   - Reset status = "pending", agentId = "", assignedAt = 0, lastHeartbeatAt = 0
//   - Method này nên được gọi định kỳ bởi background job
func (s *AIWorkflowCommandService) ReleaseStuckCommands(ctx context.Context, timeoutSeconds int64) (int64, error) {
	// Validate timeout
	if timeoutSeconds < 60 {
		timeoutSeconds = 300 // Mặc định 5 phút
	}

	now := time.Now().Unix()
	timeoutThreshold := now - timeoutSeconds

	// Filter: Tìm commands có status = "executing" và lastHeartbeatAt cũ hơn threshold
	// Hoặc không có lastHeartbeatAt và executedAt cũ hơn threshold
	filter := bson.M{
		"status": models.AIWorkflowCommandStatusExecuting,
		"$or": []bson.M{
			// Có lastHeartbeatAt nhưng quá lâu
			{
				"lastHeartbeatAt": bson.M{"$exists": true, "$lt": timeoutThreshold},
			},
			// Không có lastHeartbeatAt nhưng executedAt quá lâu (fallback cho commands cũ)
			{
				"lastHeartbeatAt": bson.M{"$exists": false},
				"executedAt":      bson.M{"$exists": true, "$lt": timeoutThreshold},
			},
		},
	}

	// Update: Reset về pending và clear agent info
	update := bson.M{
		"$set": bson.M{
			"status":          models.AIWorkflowCommandStatusPending,
			"agentId":         "",
			"assignedAt":      0,
			"lastHeartbeatAt": 0,
			"executedAt":      0, // Reset executedAt để tránh confusion
		},
		"$unset": bson.M{
			"progress": "", // Xóa progress cũ
		},
	}

	// Update nhiều documents
	result, err := s.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to release stuck commands: %w", err)
	}

	return result.ModifiedCount, nil
}
