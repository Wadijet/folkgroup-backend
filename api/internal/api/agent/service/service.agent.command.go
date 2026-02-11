package agentsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	agentmodels "meta_commerce/internal/api/agent/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// AgentCommandService xử lý logic cho agent commands
type AgentCommandService struct {
	*basesvc.BaseServiceMongoImpl[agentmodels.AgentCommand]
}

// NewAgentCommandService tạo mới AgentCommandService
func NewAgentCommandService() (*AgentCommandService, error) {
	collection, exist := global.RegistryCollections.Get("agent_commands")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_commands collection")
	}
	return &AgentCommandService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[agentmodels.AgentCommand](collection),
	}, nil
}

// GetPendingCommand lấy command pending đầu tiên cho agent
func (s *AgentCommandService) GetPendingCommand(ctx context.Context, agentId string) (map[string]interface{}, error) {
	filter := bson.M{"agentId": agentId, "status": "pending"}
	opts := options.FindOne().SetSort(bson.M{"createdAt": 1})
	command, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
	if err != nil {
		if err == common.ErrNotFound || err.Error() == "mongo: no documents in result" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending command: %w", err)
	}
	return map[string]interface{}{
		"id": command.ID.Hex(), "type": command.Type, "target": command.Target,
		"params": command.Params, "createdAt": command.CreatedAt,
	}, nil
}

// GetPendingCommands lấy danh sách tất cả commands pending cho agent
func (s *AgentCommandService) GetPendingCommands(ctx context.Context, agentId string) ([]map[string]interface{}, error) {
	filter := bson.M{"agentId": agentId, "status": "pending"}
	opts := options.Find().SetSort(bson.M{"createdAt": 1})
	commands, err := s.BaseServiceMongoImpl.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending commands: %w", err)
	}
	results := make([]map[string]interface{}, 0, len(commands))
	for _, command := range commands {
		results = append(results, map[string]interface{}{
			"id": command.ID.Hex(), "type": command.Type, "target": command.Target,
			"params": command.Params, "createdAt": command.CreatedAt,
		})
	}
	return results, nil
}

// CreateCommand tạo command mới
func (s *AgentCommandService) CreateCommand(ctx context.Context, agentId string, commandType string, target string, params map[string]interface{}, createdBy *primitive.ObjectID) (*agentmodels.AgentCommand, error) {
	now := time.Now().Unix()
	command := agentmodels.AgentCommand{
		ID: primitive.NewObjectID(), AgentID: agentId, Type: commandType, Target: target,
		Params: params, Status: "pending", CreatedBy: createdBy, CreatedAt: now,
	}
	inserted, err := s.BaseServiceMongoImpl.InsertOne(ctx, command)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &inserted, nil
}

// ReportCommandResult báo cáo kết quả thực thi command
func (s *AgentCommandService) ReportCommandResult(ctx context.Context, commandID primitive.ObjectID, status string, result map[string]interface{}, errorMsg string) error {
	filter := bson.M{"_id": commandID}
	update := bson.M{"$set": bson.M{"status": status, "result": result, "error": errorMsg, "completedAt": time.Now().Unix()}}
	_, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	return err
}

// ClaimPendingCommands claim các commands đang chờ (pending) với atomic operation
func (s *AgentCommandService) ClaimPendingCommands(ctx context.Context, agentId string, limit int) ([]agentmodels.AgentCommand, error) {
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
	filter := bson.M{"agentId": agentId, "status": "pending"}
	update := bson.M{"$set": bson.M{"status": "executing", "executedAt": now, "lastHeartbeatAt": now}}
	opts := options.FindOneAndUpdate().SetSort(bson.M{"createdAt": 1}).SetReturnDocument(options.After)
	coll := s.Collection()
	var claimedCommands []agentmodels.AgentCommand
	for i := 0; i < limit; i++ {
		var command agentmodels.AgentCommand
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
func (s *AgentCommandService) UpdateHeartbeat(ctx context.Context, commandID primitive.ObjectID, agentId string, progress map[string]interface{}) (*agentmodels.AgentCommand, error) {
	if agentId == "" {
		return nil, fmt.Errorf("agentId không được để trống")
	}
	now := time.Now().Unix()
	filter := bson.M{"_id": commandID, "agentId": agentId, "status": "executing"}
	update := bson.M{"$set": bson.M{"lastHeartbeatAt": now}}
	if progress != nil {
		update["$set"].(bson.M)["progress"] = progress
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var command agentmodels.AgentCommand
	err := s.Collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(&command)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || err == common.ErrNotFound {
			return nil, fmt.Errorf("command không tồn tại, không thuộc về agent này, hoặc đã completed/failed")
		}
		return nil, fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return &command, nil
}

// ReleaseStuckCommands giải phóng các commands bị stuck
func (s *AgentCommandService) ReleaseStuckCommands(ctx context.Context, timeoutSeconds int64) (int64, error) {
	if timeoutSeconds < 60 {
		timeoutSeconds = 300
	}
	now := time.Now().Unix()
	timeoutThreshold := now - timeoutSeconds
	filter := bson.M{
		"status": "executing",
		"$or": []bson.M{
			{"lastHeartbeatAt": bson.M{"$exists": true, "$lt": timeoutThreshold}},
			{"lastHeartbeatAt": bson.M{"$exists": false}, "executedAt": bson.M{"$exists": true, "$lt": timeoutThreshold}},
		},
	}
	update := bson.M{
		"$set": bson.M{"status": "pending", "executedAt": 0, "lastHeartbeatAt": 0},
		"$unset": bson.M{"progress": ""},
	}
	result, err := s.Collection().UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to release stuck commands: %w", err)
	}
	return result.ModifiedCount, nil
}
