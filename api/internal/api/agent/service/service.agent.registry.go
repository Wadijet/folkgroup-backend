package agentsvc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	agentmodels "meta_commerce/internal/api/agent/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentRegistryService xử lý logic cho agent registry
type AgentRegistryService struct {
	*basesvc.BaseServiceMongoImpl[agentmodels.AgentRegistry]
}

// NewAgentRegistryService tạo mới AgentRegistryService
func NewAgentRegistryService() (*AgentRegistryService, error) {
	collection, exist := global.RegistryCollections.Get("agent_registry")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_registry collection")
	}
	return &AgentRegistryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[agentmodels.AgentRegistry](collection),
	}, nil
}

// FindOrCreateByAgentID tìm hoặc tạo agent registry theo agentId
func (s *AgentRegistryService) FindOrCreateByAgentID(ctx context.Context, agentId string) (*agentmodels.AgentRegistry, error) {
	filter := bson.M{"agentId": agentId}
	agent, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err == nil {
		return &agent, nil
	}
	now := time.Now().Unix()
	newAgent := agentmodels.AgentRegistry{
		ID:            primitive.NewObjectID(),
		AgentID:       agentId,
		Status:        "offline",
		HealthStatus:  "unhealthy",
		FirstSeenAt:   now,
		LastSeenAt:    now,
		LastCheckInAt: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	inserted, err := s.BaseServiceMongoImpl.InsertOne(ctx, newAgent)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &inserted, nil
}

// UpdateByAgentID cập nhật agent registry theo agentId
func (s *AgentRegistryService) UpdateByAgentID(ctx context.Context, agentId string, updateData map[string]interface{}) (*agentmodels.AgentRegistry, error) {
	filter := bson.M{"agentId": agentId}
	update := bson.M{"$set": updateData}
	updated, err := s.BaseServiceMongoImpl.FindOneAndUpdate(ctx, filter, update, nil)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &updated, nil
}

// UpdateStatus cập nhật status và các thông tin realtime của agent
func (s *AgentRegistryService) UpdateStatus(ctx context.Context, agentRegistryID primitive.ObjectID, statusData map[string]interface{}) error {
	now := time.Now().Unix()
	filter := bson.M{"_id": agentRegistryID}
	existingAgent, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	update := bson.M{
		"$set": bson.M{
			"status":        getString(statusData, "status", existingAgent.Status),
			"healthStatus":  getString(statusData, "healthStatus", existingAgent.HealthStatus),
			"systemInfo":    getMap(statusData, "systemInfo"),
			"metrics":       getMap(statusData, "metrics"),
			"jobStatus":     getSliceMap(statusData, "jobStatus"),
			"configVersion": getInt64(statusData, "configVersion", existingAgent.ConfigVersion),
			"configHash":    getString(statusData, "configHash", existingAgent.ConfigHash),
			"lastCheckInAt": getInt64(statusData, "lastCheckInAt", existingAgent.LastCheckInAt),
			"lastSeenAt":    getInt64(statusData, "lastSeenAt", existingAgent.LastSeenAt),
			"updatedAt":     now,
		},
	}
	metadataFields := []string{"name", "displayName", "description", "botVersion", "icon", "color", "category"}
	for _, field := range metadataFields {
		if val, ok := statusData[field]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				update["$set"].(bson.M)[field] = strVal
			}
		}
	}
	if tags, ok := statusData["tags"]; ok {
		if tagsSlice, ok := tags.([]string); ok && len(tagsSlice) > 0 {
			update["$set"].(bson.M)["tags"] = tagsSlice
		} else if tagsInterface, ok := tags.([]interface{}); ok && len(tagsInterface) > 0 {
			tagsStr := make([]string, 0, len(tagsInterface))
			for _, tag := range tagsInterface {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					tagsStr = append(tagsStr, tagStr)
				}
			}
			if len(tagsStr) > 0 {
				update["$set"].(bson.M)["tags"] = tagsStr
			}
		}
	}
	if existingAgent.FirstSeenAt == 0 {
		update["$set"].(bson.M)["firstSeenAt"] = getInt64(statusData, "firstSeenAt", now)
	}
	_, err = s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	return err
}

func getString(m map[string]interface{}, key string, defaultValue string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getInt64(m map[string]interface{}, key string, defaultValue int64) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		case string:
			if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return defaultValue
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if mapVal, ok := v.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return nil
}

func getSliceMap(m map[string]interface{}, key string) []map[string]interface{} {
	if v, ok := m[key]; ok {
		if slice, ok := v.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(slice))
			for _, item := range slice {
				if mapVal, ok := item.(map[string]interface{}); ok {
					result = append(result, mapVal)
				}
			}
			return result
		}
	}
	return nil
}
