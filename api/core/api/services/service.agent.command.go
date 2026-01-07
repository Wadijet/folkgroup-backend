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
func (s *AgentCommandService) GetPendingCommand(ctx context.Context, agentRegistryID primitive.ObjectID) (map[string]interface{}, error) {
	filter := bson.M{
		"agentId": agentRegistryID,
		"status":  "pending",
	}

	opts := options.FindOne().SetSort(bson.M{"createdAt": 1})
	command, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
	if err != nil {
		// Không có command pending
		return nil, nil
	}

	// Mark as executing
	now := time.Now().Unix()
	update := bson.M{
		"$set": bson.M{
			"status":     "executing",
			"executedAt": now,
		},
	}
	s.BaseServiceMongoImpl.UpdateOne(ctx, bson.M{"_id": command.ID}, update, nil)

	// Convert to map for response
	result := map[string]interface{}{
		"id":        command.ID.Hex(),
		"type":      command.Type,
		"target":    command.Target,
		"params":    command.Params,
		"createdAt": command.CreatedAt,
	}

	return result, nil
}

// CreateCommand tạo command mới
func (s *AgentCommandService) CreateCommand(ctx context.Context, agentRegistryID primitive.ObjectID, commandType string, target string, params map[string]interface{}, createdBy *primitive.ObjectID) (*models.AgentCommand, error) {
	now := time.Now().Unix()

	command := models.AgentCommand{
		ID:        primitive.NewObjectID(),
		AgentID:   agentRegistryID,
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
