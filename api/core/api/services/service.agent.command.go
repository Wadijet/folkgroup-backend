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

// AgentCommandService xử lý logic cho agent commands
type AgentCommandService struct {
	*BaseServiceMongoImpl[models.AgentCommand]
}

// NewAgentCommandService tạo mới AgentCommandService
func NewAgentCommandService() (*AgentCommandService, error) {
	collection, exist := global.RegistryCollections.Get("agent_commands")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_commands collection")
	}

	return &AgentCommandService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AgentCommand](collection),
	}, nil
}

// GetPendingCommand lấy command pending đầu tiên cho agent
// Nhận agentId (string) - id chung giữa các collection (AgentRegistry.AgentID)
// Trả về command nếu có, nil nếu không có command pending (không phải lỗi)
// Lưu ý: Server KHÔNG tự động đổi status, bot sẽ tự update status khi nhận và execute command
// Tham số:
//   - ctx: Context
//   - agentId: AgentID (string) - id chung giữa các collection, tương ứng với AgentRegistry.AgentID
//
// Trả về:
//   - map[string]interface{}: Command dưới dạng map hoặc nil nếu không có
//   - error: Lỗi nếu có trong quá trình query
func (s *AgentCommandService) GetPendingCommand(ctx context.Context, agentId string) (map[string]interface{}, error) {
	filter := bson.M{
		"agentId": agentId, // AgentCommand.AgentID là string (id chung giữa các collection)
		"status":  "pending",
	}

	opts := options.FindOne().SetSort(bson.M{"createdAt": 1})
	command, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
	if err != nil {
		// Phân biệt giữa "không có command" (ErrNoDocuments) và lỗi thực sự
		if err == common.ErrNotFound || err.Error() == "mongo: no documents in result" {
			// Không có command pending - đây là trường hợp hợp lệ, không phải lỗi
			return nil, nil
		}
		// Lỗi thực sự - trả về error để caller có thể log
		return nil, fmt.Errorf("failed to get pending command: %w", err)
	}

	// Convert to map for response
	// Lưu ý: Không đổi status, bot sẽ tự update khi nhận và execute command
	result := map[string]interface{}{
		"id":        command.ID.Hex(),
		"type":      command.Type,
		"target":    command.Target,
		"params":    command.Params,
		"createdAt": command.CreatedAt,
	}

	return result, nil
}

// GetPendingCommands lấy danh sách tất cả commands pending cho agent
// Nhận agentId (string) - id chung giữa các collection (AgentRegistry.AgentID)
// Trả về danh sách commands (có thể rỗng)
// Lưu ý: Server KHÔNG tự động đổi status, bot sẽ tự update status khi nhận và execute command
// Tham số:
//   - ctx: Context
//   - agentId: AgentID (string) - id chung giữa các collection, tương ứng với AgentRegistry.AgentID
//
// Trả về:
//   - []map[string]interface{}: Danh sách commands dưới dạng map (rỗng nếu không có)
//   - error: Lỗi nếu có trong quá trình query
func (s *AgentCommandService) GetPendingCommands(ctx context.Context, agentId string) ([]map[string]interface{}, error) {
	filter := bson.M{
		"agentId": agentId, // AgentCommand.AgentID là string (id chung giữa các collection)
		"status":  "pending",
	}

	opts := options.Find().SetSort(bson.M{"createdAt": 1})
	commands, err := s.BaseServiceMongoImpl.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending commands: %w", err)
	}

	// Convert to map array for response
	results := make([]map[string]interface{}, 0, len(commands))
	for _, command := range commands {
		result := map[string]interface{}{
			"id":        command.ID.Hex(),
			"type":      command.Type,
			"target":    command.Target,
			"params":    command.Params,
			"createdAt": command.CreatedAt,
		}
		results = append(results, result)
	}

	return results, nil
}

// CreateCommand tạo command mới
// Tham số:
//   - ctx: Context
//   - agentId: AgentID (string) - id chung giữa các collection, tương ứng với AgentRegistry.AgentID
//   - commandType: Loại command
//   - target: Target của command
//   - params: Tham số cho command
//   - createdBy: User ID nếu admin tạo
//
// Trả về:
//   - *models.AgentCommand: Command đã được tạo
//   - error: Lỗi nếu có
func (s *AgentCommandService) CreateCommand(ctx context.Context, agentId string, commandType string, target string, params map[string]interface{}, createdBy *primitive.ObjectID) (*models.AgentCommand, error) {
	now := time.Now().Unix()

	command := models.AgentCommand{
		ID:        primitive.NewObjectID(),
		AgentID:   agentId, // AgentID là string (id chung giữa các collection)
		Type:      commandType,
		Target:    target,
		Params:    params,
		Status:    "pending",
		CreatedBy: createdBy,
		CreatedAt: now,
	}

	inserted, err := s.BaseServiceMongoImpl.InsertOne(ctx, command)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &inserted, nil
}

// ReportCommandResult báo cáo kết quả thực thi command
func (s *AgentCommandService) ReportCommandResult(ctx context.Context, commandID primitive.ObjectID, status string, result map[string]interface{}, errorMsg string) error {
	now := time.Now().Unix()

	filter := bson.M{"_id": commandID}
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"result":      result,
			"error":       errorMsg,
			"completedAt": now,
		},
	}

	_, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	return err
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
//   - []models.AgentCommand: Danh sách commands đã được claim (có thể rỗng nếu không có command pending)
//   - Commands được claim sẽ có status = "executing" và executedAt được set
//   - error: Lỗi nếu có trong quá trình claim
//
// Logic:
//   1. Tìm các commands có status = "pending" và agentId khớp
//   2. Atomic update: Set status = "executing", executedAt = now, lastHeartbeatAt = now
//   3. Trả về danh sách commands đã được claim
//
// Lưu ý:
//   - Operation này là atomic, đảm bảo không có race condition
//   - Nếu không có command pending, trả về mảng rỗng (không phải lỗi)
//   - Agent cần giải phóng command bằng cách update status khi hoàn thành hoặc thất bại
func (s *AgentCommandService) ClaimPendingCommands(ctx context.Context, agentId string, limit int) ([]models.AgentCommand, error) {
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

	// Filter: Tìm commands có status = "pending" và agentId khớp
	filter := bson.M{
		"agentId": agentId,
		"status":  "pending",
	}

	// Update: Set status = "executing", executedAt = now, lastHeartbeatAt = now
	update := bson.M{
		"$set": bson.M{
			"status":          "executing",
			"executedAt":      now,
			"lastHeartbeatAt": now, // Set heartbeat ngay khi claim
		},
	}

	// Options: Sort theo createdAt tăng dần (FIFO), limit số lượng
	opts := options.FindOneAndUpdate().
		SetSort(bson.M{"createdAt": 1}).
		SetReturnDocument(options.After) // Trả về document sau khi update

	// Claim từng command một cho đến khi đủ limit hoặc không còn command pending
	var claimedCommands []models.AgentCommand

	for i := 0; i < limit; i++ {
		var command models.AgentCommand
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
//   - *models.AgentCommand: Command đã được update
//   - error: Lỗi nếu có (ví dụ: command không tồn tại, không thuộc về agent này, hoặc đã completed/failed)
//
// Lưu ý:
//   - Chỉ update được nếu command có status = "executing" và agentId khớp
//   - Nếu command đã completed/failed, không cho phép update
func (s *AgentCommandService) UpdateHeartbeat(ctx context.Context, commandID primitive.ObjectID, agentId string, progress map[string]interface{}) (*models.AgentCommand, error) {
	// Validate agentId
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}

	now := time.Now().Unix()

	// Filter: Tìm command có ID và agentId khớp, status = "executing"
	filter := bson.M{
		"_id":     commandID,
		"agentId": agentId,
		"status":  "executing",
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

	var command models.AgentCommand
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
// Tự động reset status về "pending" và clear executedAt để các job khác có thể claim
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
//   - Reset status = "pending", executedAt = 0, lastHeartbeatAt = 0
//   - Method này nên được gọi định kỳ bởi background job
func (s *AgentCommandService) ReleaseStuckCommands(ctx context.Context, timeoutSeconds int64) (int64, error) {
	// Validate timeout
	if timeoutSeconds < 60 {
		timeoutSeconds = 300 // Mặc định 5 phút
	}

	now := time.Now().Unix()
	timeoutThreshold := now - timeoutSeconds

	// Filter: Tìm commands có status = "executing" và lastHeartbeatAt cũ hơn threshold
	// Hoặc không có lastHeartbeatAt và executedAt cũ hơn threshold
	filter := bson.M{
		"status": "executing",
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

	// Update: Reset về pending và clear timestamps
	update := bson.M{
		"$set": bson.M{
			"status":          "pending",
			"executedAt":      0,
			"lastHeartbeatAt": 0,
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
