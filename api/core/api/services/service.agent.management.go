package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	models "meta_commerce/core/api/models/mongodb"
)

// AgentManagementService xử lý logic cho bot management system
type AgentManagementService struct {
	registryService *AgentRegistryService
	configService   *AgentConfigService
	commandService  *AgentCommandService
	activityService *AgentActivityService
}

// NewAgentManagementService tạo mới AgentManagementService
func NewAgentManagementService() (*AgentManagementService, error) {
	registryService, err := NewAgentRegistryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry service: %w", err)
	}

	configService, err := NewAgentConfigService()
	if err != nil {
		return nil, fmt.Errorf("failed to create config service: %w", err)
	}

	commandService, err := NewAgentCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create command service: %w", err)
	}

	activityService, err := NewAgentActivityService()
	if err != nil {
		return nil, fmt.Errorf("failed to create activity service: %w", err)
	}

	return &AgentManagementService{
		registryService: registryService,
		configService:   configService,
		commandService:  commandService,
		activityService: activityService,
	}, nil
}

// HandleEnhancedCheckIn xử lý enhanced check-in từ bot
func (s *AgentManagementService) HandleEnhancedCheckIn(ctx context.Context, agentId string, checkInData map[string]interface{}) (map[string]interface{}, error) {
	now := time.Now().Unix()

	// 1. Tìm hoặc tạo agent registry
	agentRegistry, err := s.registryService.FindOrCreateByAgentID(ctx, agentId)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create agent registry: %w", err)
	}

	// 2. Xử lý config submit (nếu có configData trong request)
	if configData, ok := checkInData["configData"].(map[string]interface{}); ok {
		// Lưu ý: Không tính hash ở đây vì SubmitConfig sẽ cleanup metadata trước khi tính hash
		// Nếu bot gửi configHash, chỉ dùng để tham khảo, SubmitConfig sẽ tính lại sau khi cleanup
		configHash := ""
		if hash, ok := checkInData["configHash"].(string); ok {
			// Lưu hash từ bot để tham khảo, nhưng SubmitConfig sẽ tính lại sau khi cleanup metadata
			configHash = hash
		}

		// Submit config (hash sẽ được tính lại trong SubmitConfig sau khi cleanup metadata)
		// Sử dụng agentRegistry.AgentID (string) thay vì agentRegistry.ID để tương ứng với AgentConfig.AgentID
		fmt.Printf("[AgentManagement] Info: Submitting config for agent %s\n", agentRegistry.AgentID)
		submittedConfig, err := s.configService.SubmitConfig(ctx, agentRegistry.AgentID, configData, configHash, true)
		if err != nil {
			// Log error nhưng không fail check-in (bot vẫn cần nhận được response)
			fmt.Printf("[AgentManagement] Error: Failed to submit config for agent %s: %v\n", agentRegistry.AgentID, err)
		} else if submittedConfig != nil {
			// Log info để debug
			fmt.Printf("[AgentManagement] Info: Config submitted successfully for agent %s, version: %d, hash: %s, id: %s\n",
				agentRegistry.AgentID, submittedConfig.Version, submittedConfig.ConfigHash, submittedConfig.ID.Hex())
		} else {
			// Trường hợp config đã tồn tại với hash giống (không tạo version mới)
			fmt.Printf("[AgentManagement] Info: Config already exists with same hash for agent %s\n", agentRegistry.AgentID)
		}
	}

	// 3. Update agent registry với status và thông tin realtime
	// Lưu ý: Đã ghép agent_status vào agent_registry, chỉ cần update một lần
	statusData := map[string]interface{}{
		"status":        checkInData["status"],
		"healthStatus":  checkInData["healthStatus"],
		"systemInfo":    checkInData["systemInfo"],
		"metrics":       checkInData["metrics"],
		"jobStatus":     checkInData["jobStatus"],
		"configVersion": checkInData["configVersion"],
		"configHash":    checkInData["configHash"],
		"lastCheckInAt": now,
		"lastSeenAt":    now,
	}

	// 3.1. Xử lý metadata từ bot (nếu có) - chỉ update nếu giá trị mới khác rỗng
	// Lưu ý: Bot có thể gửi metadata trong check-in, nhưng admin vẫn có thể override qua CRUD endpoint
	// Chỉ update metadata nếu giá trị mới khác rỗng và agent chưa có giá trị đó (hoặc cho phép bot update)
	metadataFields := []string{"name", "displayName", "description", "botVersion", "icon", "color", "category"}
	for _, field := range metadataFields {
		if val, ok := checkInData[field]; ok {
			// Chỉ update nếu giá trị mới khác rỗng
			if strVal, ok := val.(string); ok && strVal != "" {
				statusData[field] = strVal
			}
		}
	}

	// Xử lý tags (array)
	if tags, ok := checkInData["tags"].([]interface{}); ok && len(tags) > 0 {
		tagsStr := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				tagsStr = append(tagsStr, tagStr)
			}
		}
		if len(tagsStr) > 0 {
			statusData["tags"] = tagsStr
		}
	}

	err = s.registryService.UpdateStatus(ctx, agentRegistry.ID, statusData)
	if err != nil {
		return nil, fmt.Errorf("failed to update registry status: %w", err)
	}

	// 5. Save activity log
	activityData := map[string]interface{}{
		"checkInData": checkInData,
	}
	err = s.activityService.LogActivity(ctx, agentRegistry.ID, "check_in", activityData, "info")
	if err != nil {
		// Log warning nhưng không fail
		fmt.Printf("[AgentManagement] Warning: Failed to log activity: %v\n", err)
	}

	// 6. Check pending commands
	// Sử dụng agentId (string) - id chung giữa các collection
	pendingCommands, err := s.commandService.GetPendingCommands(ctx, agentId)
	if err != nil {
		// Log error nhưng không fail check-in (bot vẫn cần nhận được response)
		fmt.Printf("[AgentManagement] Error: Failed to get pending commands for agent %s: %v\n", agentId, err)
	} else if len(pendingCommands) > 0 {
		// Log info khi có commands để debug
		fmt.Printf("[AgentManagement] Info: Found %d pending commands for agent %s\n", len(pendingCommands), agentId)
	}

	// 7. Get current config và tính diff
	// Lưu ý: GetCurrentConfig trả về nil, nil nếu không tìm thấy config (trường hợp hợp lệ)
	// Theo tài liệu BOT_MANAGEMENT_SYSTEM_PROPOSAL.md: "Nếu không có config, đơn giản là không trả về config trong response (không phải lỗi)"
	// Agent có thể chưa có config (lần đầu chạy) - đây là trường hợp hợp lệ
	// QUAN TRỌNG: Không fail check-in nếu không có config - chỉ log warning và tiếp tục
	// Đơn giản hóa: Nếu GetCurrentConfig trả về error, luôn coi như không có config (hợp lệ cho agent mới)
	// Sử dụng agentRegistry.AgentID (string) thay vì agentRegistry.ID để tương ứng với AgentConfig.AgentID
	currentConfig, err := s.configService.GetCurrentConfig(ctx, agentRegistry.AgentID)
	if err != nil {
		// GetCurrentConfig đã xử lý ErrNotFound và trả về nil, nil
		// Nhưng nếu error đã bị wrap hoặc convert, kiểm tra lại
		// Đơn giản hóa: Luôn coi như không có config nếu có error (hợp lệ cho agent mới)
		// Log warning để debug nhưng không fail check-in
		errMsg := err.Error()
		fmt.Printf("[AgentManagement] Warning: Failed to get current config (coi như không có config): %v\n", errMsg)
		currentConfig = nil
	}
	// Nếu err == nil, currentConfig đã được set đúng từ GetCurrentConfig

	// 8. Build response
	response := map[string]interface{}{
		"success":     true,
		"message":     "Check-in thành công",
		"serverTime":  now,
		"nextCheckIn": 60, // Default 60 giây
	}

	// Add commands nếu có (danh sách commands)
	if len(pendingCommands) > 0 {
		response["commands"] = pendingCommands
	}

	// Add config (với diff nếu có update)
	if currentConfig != nil {
		var botConfigVersion int64 = 0
		// Bot có thể gửi version dưới dạng string (từ check-in cũ) hoặc int64 (mới)
		if version, ok := checkInData["configVersion"].(int64); ok {
			botConfigVersion = version
		} else if versionStr, ok := checkInData["configVersion"].(string); ok {
			// Backward compatibility: parse string sang int64
			if parsed, err := strconv.ParseInt(versionStr, 10, 64); err == nil {
				botConfigVersion = parsed
			}
		}

		configResponse := s.buildConfigResponse(currentConfig, botConfigVersion)
		response["config"] = configResponse
	}

	return response, nil
}

// buildConfigResponse xây dựng config response với diff
func (s *AgentManagementService) buildConfigResponse(currentConfig *models.AgentConfig, botConfigVersion int64) map[string]interface{} {
	response := map[string]interface{}{
		"version":    currentConfig.Version,
		"configHash": currentConfig.ConfigHash,
		"hasUpdate":  false,
	}

	// Nếu version khác → có update
	if botConfigVersion != currentConfig.Version {
		// Nếu bot chưa có version (0) hoặc version không match → yêu cầu gửi full config
		if botConfigVersion == 0 {
			response["needFullConfig"] = true
		} else {
			// Tính diff (chỉ phần thay đổi)
			// TODO: Implement diff calculation
			response["hasUpdate"] = true
			response["configDiff"] = currentConfig.ConfigData // Tạm thời trả về full config, sẽ implement diff sau
		}
	}

	return response
}
