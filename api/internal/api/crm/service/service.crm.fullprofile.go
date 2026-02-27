// Package crmvc - Lấy toàn bộ thông tin khách hàng về một chỗ (profile + orders + conversations + notes + lịch sử).
package crmvc

import (
	"context"
	"fmt"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const (
	fullProfileOrderLimit     = 20
	fullProfileNoteLimit      = 50
	fullProfileActivityLimit  = 50
)

// GetFullProfileOpts tùy chọn khi gọi GetFullProfile (clientIp, userAgent, actor cho audit).
type GetFullProfileOpts struct {
	ClientIp  string
	UserAgent string
	ActorId   *primitive.ObjectID
	ActorName string
	Domains   []string // Lọc activity theo domain (rỗng = tất cả)
}

// GetFullProfile trả về toàn bộ thông tin khách: profile, đơn hàng gần đây, hội thoại, ghi chú, lịch sử hoạt động.
// Đồng thời ghi lịch sử profile_viewed vào crm_activity_history.
func (s *CrmCustomerService) GetFullProfile(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, opts *GetFullProfileOpts) (*crmdto.CrmCustomerFullProfileResponse, error) {
	// 1. Lấy profile (có thể merge từ nguồn nếu chưa có)
	profile, err := s.GetProfile(ctx, unifiedId, ownerOrgID)
	if err != nil {
		return nil, err
	}

	// 2. Lấy customer để có sourceIds cho query orders/conversations
	filter := bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}
	customer, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	// Danh sách customerId dùng để query (unifiedId + pos + fb)
	customerIds := buildCustomerIdsForQuery(&customer)

	// 3. Lấy đơn hàng gần đây
	recentOrders := s.fetchRecentOrders(ctx, customerIds, ownerOrgID)

	// 4. Lấy hội thoại
	conversations := s.fetchConversations(ctx, customerIds, ownerOrgID)

	// 5. Lấy ghi chú
	notes := s.fetchNotes(ctx, unifiedId, ownerOrgID)

	// 6. Lấy lịch sử hoạt động
	activityHistory := s.fetchActivityHistory(ctx, unifiedId, ownerOrgID, opts)

	// 7. Ghi lịch sử profile_viewed (không lưu snapshot — chỉ ghi sự kiện xem)
	if actSvc, err := NewCrmActivityService(); err == nil {
		actorName := "Hệ thống"
		if opts != nil && opts.ActorName != "" {
			actorName = opts.ActorName
		}
		metadata := map[string]interface{}{}
		_ = actSvc.LogActivity(ctx, LogActivityInput{
			UnifiedId:    unifiedId,
			OwnerOrgID:   ownerOrgID,
			Domain:       crmmodels.ActivityDomainProfile,
			ActivityType: "profile_viewed",
			Source:       "system",
			Metadata:     metadata,
			DisplayLabel: fmt.Sprintf("%s xem hồ sơ", actorName),
			DisplayIcon:  "visibility",
			ActorId:      opts.ActorId,
			ActorName:    opts.ActorName,
			ClientIp:     getOptClientIp(opts),
			UserAgent:    getOptUserAgent(opts),
		})
	}

	// currentMetrics: ưu tiên từ DB (đã lưu); fallback compute khi document cũ chưa có.
	// Đảm bảo có layer3 (derive bổ sung khi thiếu — dữ liệu cũ hoặc recalculate chưa ghi đủ).
	currentMetrics := customer.CurrentMetrics
	if len(currentMetrics) == 0 {
		currentMetrics = BuildCurrentMetricsSnapshot(&customer)
	} else {
		currentMetrics = ensureLayer3InMetrics(currentMetrics)
	}

	return &crmdto.CrmCustomerFullProfileResponse{
		Profile:         *profile,
		CurrentMetrics:  currentMetrics,
		RecentOrders:    recentOrders,
		Conversations:   conversations,
		Notes:           notes,
		ActivityHistory: activityHistory,
	}, nil
}

// buildCustomerIdsForQuery tạo danh sách customerId để query orders/conversations.
func buildCustomerIdsForQuery(c *crmmodels.CrmCustomer) []string {
	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(c.UnifiedId)
	if c.SourceIds.Pos != "" {
		add(c.SourceIds.Pos)
	}
	if c.SourceIds.Fb != "" {
		add(c.SourceIds.Fb)
	}
	return ids
}

// fetchRecentOrders lấy đơn hàng gần đây từ pc_pos_orders.
func (s *CrmCustomerService) fetchRecentOrders(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) []crmdto.CrmOrderSummary {
	if len(customerIds) == 0 {
		return []crmdto.CrmOrderSummary{}
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return []crmdto.CrmOrderSummary{}
	}

	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"customerId": bson.M{"$in": customerIds}},
			{"posData.customer.id": bson.M{"$in": customerIds}},
		},
	}
	opts := mongoopts.Find().SetLimit(fullProfileOrderLimit).SetSort(bson.D{{Key: "insertedAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return []crmdto.CrmOrderSummary{}
	}
	defer cursor.Close(ctx)

	var result []crmdto.CrmOrderSummary
	for cursor.Next(ctx) {
		var doc struct {
			OrderId    int64  `bson:"orderId"`
			Status     int    `bson:"status"`
			PageId     string `bson:"pageId"`
			InsertedAt int64  `bson:"insertedAt"`
			PosData    map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		totalAmount := getFloatFromMap(doc.PosData, "total_price_after_sub_discount")
		channel := "offline"
		if doc.PageId != "" {
			channel = "online"
		}
		createdAt := doc.InsertedAt
		if createdAt == 0 {
			createdAt = getInt64FromMap(doc.PosData, "inserted_at")
		}
		result = append(result, crmdto.CrmOrderSummary{
			OrderId:     doc.OrderId,
			TotalAmount: totalAmount,
			Status:      doc.Status,
			Channel:     channel,
			CreatedAt:   createdAt,
		})
	}
	return result
}

// fetchConversations lấy hội thoại từ fb_conversations.
// Match theo customerId (root), panCakeData.customer_id, panCakeData.customer.id, panCakeData.customers.id — đồng nhất với aggregateConversationMetricsForCustomer.
func (s *CrmCustomerService) fetchConversations(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) []crmdto.CrmConversationSummary {
	if len(customerIds) == 0 {
		return []crmdto.CrmConversationSummary{}
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return []crmdto.CrmConversationSummary{}
	}

	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID)
	opts := mongoopts.Find().SetLimit(20).SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return []crmdto.CrmConversationSummary{}
	}
	defer cursor.Close(ctx)

	var result []crmdto.CrmConversationSummary
	for cursor.Next(ctx) {
		var doc struct {
			ConversationId   string `bson:"conversationId"`
			PageId           string `bson:"pageId"`
			PanCakeUpdatedAt int64  `bson:"panCakeUpdatedAt"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		result = append(result, crmdto.CrmConversationSummary{
			ConversationId:   doc.ConversationId,
			PageId:           doc.PageId,
			PanCakeUpdatedAt: doc.PanCakeUpdatedAt,
		})
	}
	return result
}

// fetchNotes lấy ghi chú từ crm_notes.
func (s *CrmCustomerService) fetchNotes(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) []crmdto.CrmNoteSummary {
	noteSvc, err := NewCrmNoteService()
	if err != nil {
		return []crmdto.CrmNoteSummary{}
	}
	notes, err := noteSvc.FindByCustomerId(ctx, unifiedId, ownerOrgID, fullProfileNoteLimit)
	if err != nil {
		return []crmdto.CrmNoteSummary{}
	}
	result := make([]crmdto.CrmNoteSummary, 0, len(notes))
	for _, n := range notes {
		createdBy := ""
		if !n.CreatedBy.IsZero() {
			createdBy = n.CreatedBy.Hex()
		}
		result = append(result, crmdto.CrmNoteSummary{
			Id:             n.ID.Hex(),
			NoteText:       n.NoteText,
			NextAction:     n.NextAction,
			NextActionDate: n.NextActionDate,
			CreatedBy:      createdBy,
			CreatedAt:      n.CreatedAt,
		})
	}
	return result
}

// fetchActivityHistory lấy lịch sử hoạt động từ crm_activity_history.
func (s *CrmCustomerService) fetchActivityHistory(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, opts *GetFullProfileOpts) []crmdto.CrmActivitySummary {
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return []crmdto.CrmActivitySummary{}
	}
	domains := []string{}
	if opts != nil && len(opts.Domains) > 0 {
		domains = opts.Domains
	}
	activities, err := actSvc.FindByUnifiedId(ctx, unifiedId, ownerOrgID, domains, fullProfileActivityLimit)
	if err != nil {
		return []crmdto.CrmActivitySummary{}
	}
	result := make([]crmdto.CrmActivitySummary, 0, len(activities))
	for _, a := range activities {
		changes := make([]crmdto.ActivityChangeItem, 0, len(a.Changes))
		for _, c := range a.Changes {
			changes = append(changes, crmdto.ActivityChangeItem{
				Field:    c.Field,
				OldValue: c.OldValue,
				NewValue: c.NewValue,
			})
		}
		actorIdStr := ""
		if a.ActorId != nil && !a.ActorId.IsZero() {
			actorIdStr = a.ActorId.Hex()
		}
		activityAt := a.ActivityAt
		if activityAt <= 0 {
			activityAt = a.CreatedAt // Fallback cho bản ghi cũ
		}
		result = append(result, crmdto.CrmActivitySummary{
			ActivityType:   a.ActivityType,
			Domain:         a.Domain,
			ActivityAt:     activityAt,
			Source:         a.Source,
			SourceRef:      a.SourceRef,
			Metadata:       a.Metadata,
			DisplayLabel:   a.DisplayLabel,
			DisplayIcon:    a.DisplayIcon,
			DisplaySubtext: a.DisplaySubtext,
			ActorId:        actorIdStr,
			ActorName:      a.ActorName,
			Changes:        changes,
			Reason:         a.Reason,
			ClientIp:       a.ClientIp,
			UserAgent:      a.UserAgent,
			Status:         a.Status,
		})
	}
	return result
}

func getOptClientIp(opts *GetFullProfileOpts) string {
	if opts == nil {
		return ""
	}
	return opts.ClientIp
}

func getOptUserAgent(opts *GetFullProfileOpts) string {
	if opts == nil {
		return ""
	}
	return opts.UserAgent
}

func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func getInt64FromMap(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}
