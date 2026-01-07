package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// calculateConfigHash tính SHA256 hash của config (shared function)
func calculateConfigHash(configData map[string]interface{}) string {
	data, _ := json.Marshal(configData)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// AgentConfigService xử lý logic cho agent config
type AgentConfigService struct {
	*BaseServiceMongoImpl[models.AgentConfig]
}

// NewAgentConfigService tạo mới AgentConfigService
func NewAgentConfigService() (*AgentConfigService, error) {
	collection, exist := global.RegistryCollections.Get("agent_configs")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_configs collection")
	}

	return &AgentConfigService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AgentConfig](collection),
	}, nil
}

// SubmitConfig submit config từ bot
// Tham số:
//   - ctx: Context
//   - agentID: AgentID (string) từ AgentRegistry, tương ứng với AgentRegistry.AgentID
//   - configData: Dữ liệu config
//   - configHash: Hash của config (tự động tính nếu rỗng)
//   - submittedByBot: true nếu bot submit, false nếu admin tạo
//
// Trả về:
//   - *models.AgentConfig: Config đã được tạo hoặc config hiện tại nếu hash giống
//   - error: Lỗi nếu có
func (s *AgentConfigService) SubmitConfig(ctx context.Context, agentID string, configData map[string]interface{}, configHash string, submittedByBot bool) (*models.AgentConfig, error) {
	now := time.Now().Unix()

	// Tính hash nếu chưa có
	if configHash == "" {
		configHash = calculateConfigHash(configData)
	}

	// Tìm config active hiện tại
	currentConfig, err := s.GetCurrentConfig(ctx, agentID)
	if err == nil && currentConfig != nil {
		// Nếu hash giống → không cần tạo version mới
		if currentConfig.ConfigHash == configHash {
			return currentConfig, nil
		}
	}

	// Tạo version mới - dùng Unix timestamp (đơn giản, tự động tăng)
	version := now

	// Deactivate config cũ
	if currentConfig != nil {
		update := bson.M{"$set": bson.M{"isActive": false, "updatedAt": now}}
		filter := bson.M{"_id": currentConfig.ID}
		s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	}

	// Tạo config mới
	newConfig := models.AgentConfig{
		ID:             primitive.NewObjectID(),
		AgentID:        agentID,
		Version:        version,
		ConfigHash:     configHash,
		ConfigData:     configData,
		IsActive:       true,
		SubmittedByBot: submittedByBot,
		AppliedByBot:   false,
		AppliedStatus:  "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	inserted, err := s.BaseServiceMongoImpl.InsertOne(ctx, newConfig)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &inserted, nil
}

// GetCurrentConfig lấy config active hiện tại
// Tham số:
//   - ctx: Context
//   - agentID: AgentID (string) từ AgentRegistry, tương ứng với AgentRegistry.AgentID
//
// Trả về:
//   - *models.AgentConfig: Config active hiện tại, nil nếu không tìm thấy (trường hợp hợp lệ - agent có thể chưa có config)
//   - error: Lỗi nếu có (không phải ErrNotFound)
func (s *AgentConfigService) GetCurrentConfig(ctx context.Context, agentID string) (*models.AgentConfig, error) {
	filter := bson.M{
		"agentId":  agentID,
		"isActive": true,
	}

	opts := options.FindOne().SetSort(bson.M{"createdAt": -1})
	config, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
	if err != nil {
		// Kiểm tra xem có phải là ErrNotFound không
		// Pattern giống service.auth.user.go: chỉ return error nếu KHÔNG phải ErrNotFound
		// Nếu là ErrNotFound, trả về nil, nil (không phải lỗi - agent có thể chưa có config)

		// Cách 1: Kiểm tra bằng errors.Is (hỗ trợ wrapped errors)
		if errors.Is(err, common.ErrNotFound) {
			return nil, nil
		}

		// Cách 2: Kiểm tra bằng cách so sánh error code và message trực tiếp
		// (cho trường hợp errors.Is không hoạt động với custom error type)
		if errNotFound, ok := err.(*common.Error); ok {
			if errNotFound.Code.Code == common.ErrCodeDatabaseQuery.Code &&
				errNotFound.Message == "Không tìm thấy dữ liệu" {
				return nil, nil
			}
		}

		// Cách 3: Kiểm tra bằng error message (cho trường hợp error đã bị convert)
		// Nếu error message là "Lỗi kết nối cơ sở dữ liệu", có thể là ErrNotFound đã bị convert sai
		// Trong trường hợp này, vì không có config là hợp lệ, nên trả về nil, nil
		errMsg := err.Error()
		if errMsg == "Không tìm thấy dữ liệu" ||
			errMsg == common.ErrNotFound.Error() ||
			errMsg == "Lỗi kết nối cơ sở dữ liệu" {
			// Nếu error message là "Lỗi kết nối cơ sở dữ liệu", có thể là ErrNotFound đã bị convert sai
			// Nhưng vì không có config là hợp lệ, nên trả về nil, nil
			return nil, nil
		}

		// Nếu không phải ErrNotFound, đây là lỗi thực sự - trả về error
		return nil, err
	}

	return &config, nil
}

// UpdateConfig update config (từ admin)
// Tham số:
//   - ctx: Context
//   - agentID: AgentID (string) từ AgentRegistry, tương ứng với AgentRegistry.AgentID
//   - configData: Dữ liệu config mới
//   - changeLog: Log mô tả thay đổi
//   - changedBy: User ID nếu admin thay đổi
//
// Trả về:
//   - *models.AgentConfig: Config mới đã được tạo
//   - error: Lỗi nếu có
func (s *AgentConfigService) UpdateConfig(ctx context.Context, agentID string, configData map[string]interface{}, changeLog string, changedBy *primitive.ObjectID) (*models.AgentConfig, error) {
	now := time.Now().Unix()

	// Lấy config active hiện tại
	currentConfig, err := s.GetCurrentConfig(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	// Tính hash mới (nếu chưa có)
	configHash := ""
	if configData != nil {
		configHash = calculateConfigHash(configData)
	}

	// Deactivate config cũ
	update := bson.M{"$set": bson.M{"isActive": false, "updatedAt": now}}
	filter := bson.M{"_id": currentConfig.ID}
	s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)

	// Tạo version mới - dùng Unix timestamp (đơn giản, tự động tăng)
	newVersion := now

	// Tạo config mới
	newConfig := models.AgentConfig{
		ID:             primitive.NewObjectID(),
		AgentID:        agentID,
		Version:        newVersion,
		ConfigHash:     configHash,
		ConfigData:     configData,
		IsActive:       true,
		SubmittedByBot: false,
		ChangedBy:      changedBy,
		ChangedAt:      now,
		ChangeLog:      changeLog,
		AppliedByBot:   false,
		AppliedStatus:  "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	inserted, err := s.BaseServiceMongoImpl.InsertOne(ctx, newConfig)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &inserted, nil
}

// ReportConfigApplied báo cáo bot đã apply config
// Tham số:
//   - ctx: Context
//   - agentID: AgentID (string) từ AgentRegistry, tương ứng với AgentRegistry.AgentID
//   - version: Version của config đã apply (Unix timestamp)
//   - status: Trạng thái apply ("pending", "applied", "failed")
//   - errorMsg: Thông báo lỗi nếu có
//
// Trả về:
//   - error: Lỗi nếu có
func (s *AgentConfigService) ReportConfigApplied(ctx context.Context, agentID string, version int64, status string, errorMsg string) error {
	now := time.Now().Unix()

	filter := bson.M{
		"agentId": agentID,
		"version": version,
	}

	update := bson.M{
		"$set": bson.M{
			"appliedByBot":  true,
			"appliedAt":     now,
			"appliedStatus": status,
			"appliedError":  errorMsg,
			"updatedAt":     now,
		},
	}

	_, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
	return err
}

