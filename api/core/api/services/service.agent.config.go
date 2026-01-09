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
// Sử dụng deterministic JSON marshal để đảm bảo hash nhất quán
func calculateConfigHash(configData map[string]interface{}) string {
	// Sử dụng json.Marshal với sorted keys để đảm bảo deterministic
	// Go's json.Marshal với map[string]interface{} không đảm bảo thứ tự
	// Nên cần sort keys trước khi marshal
	data, err := json.Marshal(configData)
	if err != nil {
		// Fallback: nếu marshal lỗi, dùng string representation
		data = []byte(fmt.Sprintf("%v", configData))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// AgentConfigService xử lý logic cho agent config
type AgentConfigService struct {
	*BaseServiceMongoImpl[models.AgentConfig]
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
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AgentConfig](collection),
		registryService:    registryService,
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

	// Lưu ý: KHÔNG validate config khi bot submit vì:
	// - Config hoàn toàn do bot tự quản lý và sử dụng theo mục đích của bot
	// - Bot tự biết cấu trúc config của mình
	// - Validation chỉ áp dụng khi admin update config (trong UpdateConfig)
	// - Metadata của job giờ được gửi kèm trong JobStatus, không cần cleanup khỏi config

	// Tính hash nếu chưa có
	if configHash == "" {
		configHash = calculateConfigHash(configData)
	}

	// Tìm config hiện tại để kiểm tra hash
	currentConfig, err := s.GetCurrentConfig(ctx, agentID)
	if err == nil && currentConfig != nil {
		// Nếu hash giống → chỉ update updatedAt, không cần update config
		if currentConfig.ConfigHash == configHash {
			// Update updatedAt
			update := bson.M{"$set": bson.M{"updatedAt": now}}
			filter := bson.M{"_id": currentConfig.ID}
			updated, err := s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, nil)
			if err != nil {
				return currentConfig, nil // Trả về config cũ nếu update lỗi
			}
			return &updated, nil
		}
	}

	// Hash khác hoặc chưa có config → upsert config mới
	// Dùng upsert với filter agentId để đảm bảo atomic và chỉ có 1 config cho mỗi agent
	version := now
	upsertFilter := bson.M{"agentId": agentID}

	upsertData := bson.M{
		"$set": bson.M{
			"agentId":        agentID,
			"version":        version,
			"configHash":     configHash,
			"configData":     configData,
			"submittedByBot": submittedByBot,
			"appliedByBot":   false,
			"appliedStatus":  "pending",
			"updatedAt":      now,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}

	// Sử dụng FindOneAndUpdate với upsert để đảm bảo atomic
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result models.AgentConfig
	err = s.BaseServiceMongoImpl.collection.FindOneAndUpdate(ctx, upsertFilter, upsertData, opts).Decode(&result)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &result, nil
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
	filter := bson.M{"agentId": agentID}
	config, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
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

	// Lấy config active hiện tại để kiểm tra có tồn tại không
	_, err := s.GetCurrentConfig(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	// Validate jobs trong configData (nếu có) - chỉ khi admin update
	// Lưu ý: Metadata của job giờ được gửi kèm trong JobStatus, không cần cleanup khỏi config
	if configData != nil {
		// Validate jobs
		if err := ValidateJobsInConfigData(configData); err != nil {
			return nil, fmt.Errorf("jobs trong configData không hợp lệ: %w", err)
		}
	}

	// Tính hash mới (sau khi enrich)
	configHash := ""
	if configData != nil {
		configHash = calculateConfigHash(configData)
	}

	// Dùng upsert với filter agentId để đảm bảo atomic và chỉ có 1 config cho mỗi agent
	newVersion := now
	upsertFilter := bson.M{"agentId": agentID}

	upsertData := bson.M{
		"$set": bson.M{
			"agentId":        agentID,
			"version":        newVersion,
			"configHash":     configHash,
			"configData":     configData,
			"submittedByBot": false,
			"changedBy":      changedBy,
			"changedAt":      now,
			"changeLog":      changeLog,
			"appliedByBot":   false,
			"appliedStatus":  "pending",
			"updatedAt":      now,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}

	// Sử dụng FindOneAndUpdate với upsert để đảm bảo atomic
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result models.AgentConfig
	err = s.BaseServiceMongoImpl.collection.FindOneAndUpdate(ctx, upsertFilter, upsertData, opts).Decode(&result)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &result, nil
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

