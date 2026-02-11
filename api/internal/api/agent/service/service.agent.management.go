package agentsvc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	agentmodels "meta_commerce/internal/api/agent/models"
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
	agentRegistry, err := s.registryService.FindOrCreateByAgentID(ctx, agentId)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create agent registry: %w", err)
	}
	if configData, ok := checkInData["configData"].(map[string]interface{}); ok {
		configHash := ""
		if hash, ok := checkInData["configHash"].(string); ok {
			configHash = hash
		}
		fmt.Printf("[AgentManagement] Info: Submitting config for agent %s\n", agentRegistry.AgentID)
		submittedConfig, err := s.configService.SubmitConfig(ctx, agentRegistry.AgentID, configData, configHash, true)
		if err != nil {
			fmt.Printf("[AgentManagement] Error: Failed to submit config for agent %s: %v\n", agentRegistry.AgentID, err)
		} else if submittedConfig != nil {
			fmt.Printf("[AgentManagement] Info: Config submitted successfully for agent %s, version: %d, hash: %s, id: %s\n",
				agentRegistry.AgentID, submittedConfig.Version, submittedConfig.ConfigHash, submittedConfig.ID.Hex())
		} else {
			fmt.Printf("[AgentManagement] Info: Config already exists with same hash for agent %s\n", agentRegistry.AgentID)
		}
	}
	statusData := map[string]interface{}{
		"status": checkInData["status"], "healthStatus": checkInData["healthStatus"],
		"systemInfo": checkInData["systemInfo"], "metrics": checkInData["metrics"],
		"jobStatus": checkInData["jobStatus"], "configVersion": checkInData["configVersion"],
		"configHash": checkInData["configHash"], "lastCheckInAt": now, "lastSeenAt": now,
	}
	metadataFields := []string{"name", "displayName", "description", "botVersion", "icon", "color", "category"}
	for _, field := range metadataFields {
		if val, ok := checkInData[field]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				statusData[field] = strVal
			}
		}
	}
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
	activityData := map[string]interface{}{"checkInData": checkInData}
	_ = s.activityService.LogActivity(ctx, agentRegistry.ID, "check_in", activityData, "info")
	pendingCommands, err := s.commandService.GetPendingCommands(ctx, agentId)
	if err != nil {
		fmt.Printf("[AgentManagement] Error: Failed to get pending commands for agent %s: %v\n", agentId, err)
	} else if len(pendingCommands) > 0 {
		fmt.Printf("[AgentManagement] Info: Found %d pending commands for agent %s\n", len(pendingCommands), agentId)
	}
	currentConfig, err := s.configService.GetCurrentConfig(ctx, agentRegistry.AgentID)
	if err != nil {
		fmt.Printf("[AgentManagement] Warning: Failed to get current config (coi như không có config): %v\n", err.Error())
		currentConfig = nil
	}
	response := map[string]interface{}{
		"success": true, "message": "Check-in thành công", "serverTime": now, "nextCheckIn": 60,
	}
	if len(pendingCommands) > 0 {
		response["commands"] = pendingCommands
	}
	if currentConfig != nil {
		var botConfigVersion int64 = 0
		if version, ok := checkInData["configVersion"].(int64); ok {
			botConfigVersion = version
		} else if versionStr, ok := checkInData["configVersion"].(string); ok {
			if parsed, err := strconv.ParseInt(versionStr, 10, 64); err == nil {
				botConfigVersion = parsed
			}
		}
		response["config"] = s.buildConfigResponse(currentConfig, botConfigVersion)
	}
	return response, nil
}

func (s *AgentManagementService) buildConfigResponse(currentConfig *agentmodels.AgentConfig, botConfigVersion int64) map[string]interface{} {
	response := map[string]interface{}{
		"version": currentConfig.Version, "configHash": currentConfig.ConfigHash, "hasUpdate": false,
	}
	if botConfigVersion != currentConfig.Version {
		if botConfigVersion == 0 {
			response["needFullConfig"] = true
		} else {
			response["hasUpdate"] = true
			response["configDiff"] = currentConfig.ConfigData
		}
	}
	return response
}
