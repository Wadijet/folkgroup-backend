package notification

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Route đại diện cho một route từ routing rule
type Route struct {
	OrganizationID primitive.ObjectID
	ChannelID      primitive.ObjectID
}

// Router xử lý việc tìm routing rules và tạo routes
type Router struct {
	routingService *services.NotificationRoutingService
	channelService *services.NotificationChannelService
}

// NewRouter tạo mới Router
func NewRouter() (*Router, error) {
	routingService, err := services.NewNotificationRoutingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create routing service: %w", err)
	}

	channelService, err := services.NewNotificationChannelService()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel service: %w", err)
	}

	return &Router{
		routingService: routingService,
		channelService: channelService,
	}, nil
}

// FindRoutes tìm tất cả routes cho một eventType
// Hỗ trợ routing theo EventType (cụ thể) hoặc Domain (tổng quát)
// Có thể filter theo Severity để tránh spam
// Lưu ý: Chỉ tìm rules của organization trigger event (hoặc system rules)
func (r *Router) FindRoutes(ctx context.Context, eventType string, domain string, severity string, organizationID *primitive.ObjectID) ([]Route, error) {
	rules := []models.NotificationRoutingRule{}

	// 1. Tìm rules theo EventType (nếu có)
	eventTypeRules, err := r.routingService.FindByEventType(ctx, eventType, organizationID)
	if err != nil {
		// Log error nhưng tiếp tục với domain search
		fmt.Printf("🔔 [NOTIFICATION] Error finding rules by eventType '%s': %v\n", eventType, err)
	} else {
		fmt.Printf("🔔 [NOTIFICATION] Found %d rules by eventType '%s' for organization %v\n", len(eventTypeRules), eventType, organizationID)
		rules = append(rules, eventTypeRules...)
	}

	// 2. Tìm rules theo Domain (nếu có)
	if domain != "" {
		domainRules, err := r.routingService.FindByDomain(ctx, domain, organizationID)
		if err == nil {
			rules = append(rules, domainRules...)
		}
	}

	// 3. Filter theo Severity và loại bỏ duplicate
	filteredRules := []models.NotificationRoutingRule{}
	seenRuleIDs := make(map[string]bool)

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		// Loại bỏ duplicate (cùng rule ID)
		ruleID := rule.ID.Hex()
		if seenRuleIDs[ruleID] {
			continue
		}
		seenRuleIDs[ruleID] = true

		// Nếu rule có filter Severity, kiểm tra
		if len(rule.Severities) > 0 {
			severityMatched := false
			for _, s := range rule.Severities {
				if s == severity {
					severityMatched = true
					break
				}
			}
			if !severityMatched {
				continue // Bỏ qua rule này (severity không match)
			}
		}

		filteredRules = append(filteredRules, rule)
	}

	// 4. Tạo routes từ filtered rules
	routes := []Route{}

	for _, rule := range filteredRules {
		// Với mỗi team trong rule
		for _, orgID := range rule.OrganizationIDs {
			// Lấy TẤT CẢ channels của team (filter theo ChannelTypes nếu có)
			channels, err := r.channelService.FindByOrganizationID(ctx, orgID, rule.ChannelTypes)
			if err != nil {
				// Log error nhưng tiếp tục với team khác
				fmt.Printf("🔔 [NOTIFICATION] Error finding channels for orgID %s: %v\n", orgID.Hex(), err)
				continue
			}

			fmt.Printf("🔔 [NOTIFICATION] Found %d channels for orgID %s (ChannelTypes: %v)\n", len(channels), orgID.Hex(), rule.ChannelTypes)

			// Với mỗi channel của team
			for _, channel := range channels {
				if !channel.IsActive {
					fmt.Printf("🔔 [NOTIFICATION] Channel %s is not active, skipping\n", channel.ID.Hex())
					continue
				}
				fmt.Printf("🔔 [NOTIFICATION] Adding route: orgID=%s, channelID=%s, channelType=%s\n", orgID.Hex(), channel.ID.Hex(), channel.ChannelType)
				routes = append(routes, Route{
					OrganizationID: orgID,
					ChannelID:      channel.ID,
				})
			}
		}
	}

	fmt.Printf("🔔 [NOTIFICATION] Total routes found: %d\n", len(routes))

	return routes, nil
}

