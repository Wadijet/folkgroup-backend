package services

import (
	"context"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
