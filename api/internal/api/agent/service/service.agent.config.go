package agentsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	agentmodels "meta_commerce/internal/api/agent/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

func calculateConfigHash(configData map[string]interface{}) string {
	data, err := json.Marshal(configData)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", configData))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// AgentConfigService xử lý logic cho agent config
type AgentConfigService struct {
	*basesvc.BaseServiceMongoImpl[agentmodels.AgentConfig]
	registryService *AgentRegistryService
}

// NewAgentConfigService tạo mới AgentConfigService
func NewAgentConfigService() (*AgentConfigService, error) {
	collection, exist := global.RegistryCollections.Get("agent_configs")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_configs collection")
	}
	registryService, err := NewAgentRegistryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry service: %w", err)
	}
	return &AgentConfigService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[agentmodels.AgentConfig](collection),
		registryService:      registryService,
	}, nil
}

// SubmitConfig submit config từ bot
func (s *AgentConfigService) SubmitConfig(ctx context.Context, agentID string, configData map[string]interface{}, configHash string, submittedByBot bool) (*agentmodels.AgentConfig, error) {
	now := time.Now().Unix()
	if configHash == "" {
		configHash = calculateConfigHash(configData)
	}
	currentConfig, err := s.GetCurrentConfig(ctx, agentID)
	if err == nil && currentConfig != nil {
		if currentConfig.ConfigHash == configHash {
			update := bson.M{"$set": bson.M{"updatedAt": now}}
			filter := bson.M{"_id": currentConfig.ID}
			updated, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
			if err != nil {
				return currentConfig, nil
			}
			return &updated, nil
		}
	}
	version := now
	upsertFilter := bson.M{"agentId": agentID}
	upsertData := bson.M{
		"$set": bson.M{
			"agentId": agentID, "version": version, "configHash": configHash, "configData": configData,
			"submittedByBot": submittedByBot, "appliedByBot": false, "appliedStatus": "pending", "updatedAt": now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result agentmodels.AgentConfig
	err = s.Collection().FindOneAndUpdate(ctx, upsertFilter, upsertData, opts).Decode(&result)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &result, nil
}

// GetCurrentConfig lấy config active hiện tại
func (s *AgentConfigService) GetCurrentConfig(ctx context.Context, agentID string) (*agentmodels.AgentConfig, error) {
	filter := bson.M{"agentId": agentID}
	config, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, nil
		}
		if errNotFound, ok := err.(*common.Error); ok {
			if errNotFound.Code.Code == common.ErrCodeDatabaseQuery.Code && errNotFound.Message == "Không tìm thấy dữ liệu" {
				return nil, nil
			}
		}
		errMsg := err.Error()
		if errMsg == "Không tìm thấy dữ liệu" || errMsg == common.ErrNotFound.Error() || errMsg == "Lỗi kết nối cơ sở dữ liệu" {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// UpdateConfig update config (từ admin)
func (s *AgentConfigService) UpdateConfig(ctx context.Context, agentID string, configData map[string]interface{}, changeLog string, changedBy *primitive.ObjectID) (*agentmodels.AgentConfig, error) {
	now := time.Now().Unix()
	_, err := s.GetCurrentConfig(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}
	if configData != nil {
		if err := ValidateJobsInConfigData(configData); err != nil {
			return nil, fmt.Errorf("jobs trong configData không hợp lệ: %w", err)
		}
	}
	configHash := ""
	if configData != nil {
		configHash = calculateConfigHash(configData)
	}
	newVersion := now
	upsertFilter := bson.M{"agentId": agentID}
	upsertData := bson.M{
		"$set": bson.M{
			"agentId": agentID, "version": newVersion, "configHash": configHash, "configData": configData,
			"submittedByBot": false, "changedBy": changedBy, "changedAt": now, "changeLog": changeLog,
			"appliedByBot": false, "appliedStatus": "pending", "updatedAt": now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result agentmodels.AgentConfig
	err = s.Collection().FindOneAndUpdate(ctx, upsertFilter, upsertData, opts).Decode(&result)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &result, nil
}

// ReportConfigApplied báo cáo bot đã apply config
func (s *AgentConfigService) ReportConfigApplied(ctx context.Context, agentID string, version int64, status string, errorMsg string) error {
	now := time.Now().Unix()
	filter := bson.M{"agentId": agentID, "version": version}
	update := bson.M{"$set": bson.M{"appliedByBot": true, "appliedAt": now, "appliedStatus": status, "appliedError": errorMsg, "updatedAt": now}}
	_, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	return err
}
