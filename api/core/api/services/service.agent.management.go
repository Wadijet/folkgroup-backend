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
		configHash := ""
		if hash, ok := checkInData["configHash"].(string); ok {
			configHash = hash
		} else {
			// Tính hash từ configData
			configHash = calculateConfigHash(configData)
		}

		// Submit config (hash sẽ được tính trong SubmitConfig nếu chưa có)
		// Sử dụng agentRegistry.AgentID (string) thay vì agentRegistry.ID để tương ứng với AgentConfig.AgentID
		_, err := s.configService.SubmitConfig(ctx, agentRegistry.AgentID, configData, configHash, true)
		if err != nil {
			// Log warning nhưng không fail check-in
			fmt.Printf("[AgentManagement] Warning: Failed to submit config: %v\n", err)
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
