// Package reportsvc - Inbox Operations (Tab 7): KPI, bảng hội thoại, Sale performance, Alert zone.
// Data source: fb_conversations, fb_message_items, fb_pages, pc_pos_orders (conversion).
package reportsvc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BacklogWaitingCriticalMinutes ngưỡng CRITICAL: backlog > 30 phút.
const BacklogWaitingCriticalMinutes = 30

// GetInboxSnapshot trả về snapshot Tab 7 Inbox Operations.
// Bao gồm: pages, summary (6 KPI), conversations, salePerformance, alerts.
func (s *ReportService) GetInboxSnapshot(ctx context.Context, ownerOrganizationID primitive.ObjectID, params *reportdto.InboxQueryParams) (*reportdto.InboxSnapshotResult, error) {
	if params == nil {
		params = &reportdto.InboxQueryParams{}
	}
	applyInboxDefaults(params)

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	todayEnd := todayStart.Add(24*time.Hour - time.Second)

	// 1. Load pages để filter
	pages, err := s.loadFbPagesForInbox(ctx, ownerOrganizationID)
	if err != nil {
		return nil, fmt.Errorf("load pages: %w", err)
	}

	// 2. Load conversations
	convs, err := s.loadConversationsForInbox(ctx, ownerOrganizationID, params.PageID)
	if err != nil {
		return nil, fmt.Errorf("load conversations: %w", err)
	}

	// 3. Load response times từ fb_message_items (theo conversationId)
	responseTimes, err := s.loadResponseTimesForConversations(ctx, ownerOrganizationID, convs)
	if err != nil {
		return nil, fmt.Errorf("load response times: %w", err)
	}

	// 4. Load conversion (customerId → có đơn completed trong period)
	fromTime, toTime := parseInboxPeriod(params.Period, loc)
	convertedCustomers, err := s.loadConvertedCustomers(ctx, ownerOrganizationID, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("load converted customers: %w", err)
	}

	// 5. Build page names map
	pageNames := make(map[string]string)
	for _, p := range pages {
		pageNames[p.PageID] = p.PageName
	}

	// 6. Build items, KPI, alerts
	var items []reportdto.InboxConversationItem
	var convToday, backlogCount, unassignedCount int64
	var responseMins []float64

	for _, c := range convs {
		item, isBacklog, isUnassigned, _, respMin := buildInboxConversationItem(c, pageNames[c.PageId], responseTimes[c.ConversationId])
		items = append(items, item)

		if c.UpdatedAt >= todayStart.Unix() && c.UpdatedAt <= todayEnd.Unix() {
			convToday++
		}
		if isBacklog {
			backlogCount++
		}
		if isUnassigned {
			unassignedCount++
		}
		if respMin >= 0 {
			responseMins = append(responseMins, respMin)
		}
	}

	// 7. Filter items theo params.Filter
	items = filterInboxConversations(items, params.Filter)
	// 8. Sort
	sortInboxConversations(items, params.Sort)
	// 9. Paginate
	items = paginateInboxItems(items, params.Offset, params.Limit)

	// 10. KPI: median, P90
	medianResp := medianFloat64(responseMins)
	p90Resp := percentileFloat64(responseMins, 90)

	// 11. Conversion rate
	totalConvsInPeriod := int64(len(convs))
	convertedCount := int64(0)
	for _, c := range convs {
		if c.CustomerId != "" && convertedCustomers[c.CustomerId] {
			convertedCount++
		}
	}
	conversionRate := float64(0)
	if totalConvsInPeriod > 0 {
		conversionRate = float64(convertedCount) / float64(totalConvsInPeriod)
	}

	// 12. Sale performance
	salePerf := s.buildSalePerformance(convs, responseTimes, convertedCustomers)

	// 13. Alerts
	alerts := s.buildInboxAlerts(convs, pageNames)

	return &reportdto.InboxSnapshotResult{
		Pages:   pages,
		Summary: reportdto.InboxSummary{
			ConversationsToday: convToday,
			BacklogCount:       backlogCount,
			MedianResponseMin: medianResp,
			P90ResponseMin:    p90Resp,
			UnassignedCount:    unassignedCount,
			ConversionRate:     conversionRate,
		},
		Conversations:   items,
		SalePerformance: salePerf,
		Alerts:          alerts,
	}, nil
}

func applyInboxDefaults(p *reportdto.InboxQueryParams) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Filter == "" {
		p.Filter = "all"
	}
	if p.Sort == "" {
		p.Sort = "updated_desc"
	}
	if p.Period == "" {
		p.Period = "month"
	}
}

func parseInboxPeriod(period string, loc *time.Location) (from, to time.Time) {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch period {
	case "day":
		from = today
		to = today
	case "week":
		from = today.AddDate(0, 0, -7)
		to = today
	case "60d":
		from = today.AddDate(0, 0, -60)
		to = today
	case "90d":
		from = today.AddDate(0, 0, -90)
		to = today
	default:
		from = today.AddDate(0, 0, -30)
		to = today
	}
	return from, to
}

// inboxConvData dữ liệu conversation đã parse.
type inboxConvData struct {
	ConversationId string
	PageId         string
	CustomerId     string
	CustomerName   string
	UpdatedAt      int64
	PanCakeData    map[string]interface{}
}

func (s *ReportService) loadFbPagesForInbox(ctx context.Context, ownerOrgID primitive.ObjectID) ([]reportdto.InboxPageOption, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbPages)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.FbPages, common.ErrNotFound)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	opts := options.Find().SetProjection(bson.M{"pageId": 1, "pageName": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var result []reportdto.InboxPageOption
	for cursor.Next(ctx) {
		var doc struct {
			PageId   string `bson:"pageId"`
			PageName string `bson:"pageName"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.PageId == "" {
			continue
		}
		if doc.PageName == "" {
			doc.PageName = doc.PageId
		}
		result = append(result, reportdto.InboxPageOption{PageID: doc.PageId, PageName: doc.PageName})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if result == nil {
		result = []reportdto.InboxPageOption{}
	}
	return result, nil
}

func (s *ReportService) loadConversationsForInbox(ctx context.Context, ownerOrgID primitive.ObjectID, pageID string) ([]inboxConvData, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.FbConvesations, common.ErrNotFound)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	if pageID != "" {
		filter["pageId"] = pageID
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}}).
		SetLimit(500)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	// Load customer names từ fb_customers
	custColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		custColl = nil
	}
	custNames := make(map[string]string)
	if custColl != nil {
		// Sẽ fill sau khi có danh sách customerId
		_ = custColl
	}

	var result []inboxConvData
	for cursor.Next(ctx) {
		var doc struct {
			ConversationId   string                 `bson:"conversationId"`
			PageId           string                 `bson:"pageId"`
			CustomerId       string                 `bson:"customerId"`
			UpdatedAt        int64                  `bson:"updatedAt"`
			PanCakeUpdatedAt int64                  `bson:"panCakeUpdatedAt"`
			PanCakeData      map[string]interface{} `bson:"panCakeData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		updatedAt := doc.UpdatedAt
		if doc.PanCakeUpdatedAt > 0 {
			updatedAt = doc.PanCakeUpdatedAt
		}
		customerName := extractCustomerNameFromPanCake(doc.PanCakeData)
		if customerName == "" && doc.CustomerId != "" && custNames[doc.CustomerId] != "" {
			customerName = custNames[doc.CustomerId]
		}
		if customerName == "" {
			customerName = "Khách"
		}
		result = append(result, inboxConvData{
			ConversationId: doc.ConversationId,
			PageId:        doc.PageId,
			CustomerId:    doc.CustomerId,
			CustomerName:  customerName,
			UpdatedAt:     updatedAt,
			PanCakeData:   doc.PanCakeData,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if result == nil {
		result = []inboxConvData{}
	}
	return result, nil
}

func extractCustomerNameFromPanCake(pc map[string]interface{}) string {
	if pc == nil {
		return ""
	}
	// panCakeData.customer.name hoặc customers[0].name
	if cust, ok := pc["customer"].(map[string]interface{}); ok {
		if n, ok := cust["name"].(string); ok && n != "" {
			return n
		}
	}
	if arr, ok := pc["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			if n, ok := m["name"].(string); ok && n != "" {
				return n
			}
		}
	}
	return ""
}

// isLastSentByCustomer kiểm tra tin cuối từ khách (email @facebook.com).
func isLastSentByCustomer(pc map[string]interface{}) bool {
	if pc == nil {
		return false
	}
	lsb, ok := pc["last_sent_by"].(map[string]interface{})
	if !ok {
		return false
	}
	email, _ := lsb["email"].(string)
	return strings.Contains(email, "@facebook.com")
}

// isCurrentAssignEmpty kiểm tra current_assign_users rỗng.
func isCurrentAssignEmpty(pc map[string]interface{}) bool {
	if pc == nil {
		return true
	}
	arr, ok := pc["current_assign_users"].([]interface{})
	if !ok || len(arr) == 0 {
		return true
	}
	return false
}

// extractTags lấy tags text từ panCakeData.
func extractTags(pc map[string]interface{}) []string {
	if pc == nil {
		return nil
	}
	arr, ok := pc["tags"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, t := range arr {
		if m, ok := t.(map[string]interface{}); ok {
			if txt, ok := m["text"].(string); ok && txt != "" {
				out = append(out, txt)
			}
		}
	}
	return out
}

// extractAssignedSaleName lấy tên sale assign từ current_assign_users hoặc last_sent_by.
func extractAssignedSaleName(pc map[string]interface{}) string {
	if pc == nil {
		return ""
	}
	arr, ok := pc["current_assign_users"].([]interface{})
	if ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			if n, ok := m["name"].(string); ok && n != "" {
				return n
			}
		}
	}
	lsb, ok := pc["last_sent_by"].(map[string]interface{})
	if ok {
		if n, ok := lsb["admin_name"].(string); ok && n != "" {
			return n
		}
	}
	return ""
}

// extractLastMessageSnippet lấy snippet tin nhắn cuối từ panCakeData.
func extractLastMessageSnippet(pc map[string]interface{}) string {
	if pc == nil {
		return ""
	}
	msg, ok := pc["last_message"].(map[string]interface{})
	if !ok {
		return ""
	}
	// message.text hoặc content
	if t, ok := msg["text"].(string); ok && t != "" {
		if len(t) > 80 {
			return t[:80] + "..."
		}
		return t
	}
	if c, ok := msg["content"].(string); ok && c != "" {
		if len(c) > 80 {
			return c[:80] + "..."
		}
		return c
	}
	return ""
}

func (s *ReportService) loadResponseTimesForConversations(ctx context.Context, ownerOrgID primitive.ObjectID, convs []inboxConvData) (map[string]float64, error) {
	if len(convs) == 0 {
		return make(map[string]float64), nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbMessageItems)
	if !ok {
		return make(map[string]float64), nil
	}

	result := make(map[string]float64)
	convIds := make([]string, 0, len(convs))
	for _, c := range convs {
		convIds = append(convIds, c.ConversationId)
	}

	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"conversationId":     bson.M{"$in": convIds},
	}
	opts := options.Find().SetSort(bson.D{{Key: "insertedAt", Value: 1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return result, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	// Group messages by conversationId
	byConv := make(map[string][]struct {
		InsertedAt int64
		IsFromCust bool
	})
	for cursor.Next(ctx) {
		var doc struct {
			ConversationId string                 `bson:"conversationId"`
			InsertedAt     int64                  `bson:"insertedAt"`
			MessageData   map[string]interface{} `bson:"messageData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		from, _ := doc.MessageData["from"].(map[string]interface{})
		email, _ := from["email"].(string)
		isFromCust := strings.Contains(email, "@facebook.com")
		insertedAt := doc.InsertedAt
		if insertedAt == 0 {
			// Fallback: parse từ messageData.inserted_at
			if ins, ok := doc.MessageData["inserted_at"].(string); ok {
				if t, err := time.Parse("2006-01-02T15:04:05.000000", ins); err == nil {
					insertedAt = t.Unix()
				}
			}
		}
		byConv[doc.ConversationId] = append(byConv[doc.ConversationId], struct {
			InsertedAt int64
			IsFromCust bool
		}{InsertedAt: insertedAt, IsFromCust: isFromCust})
	}
	if err := cursor.Err(); err != nil {
		return result, common.ConvertMongoError(err)
	}

	// Tính response time: tin khách -> tin page kế tiếp
	for convId, msgs := range byConv {
		var lastCustAt int64
		for _, m := range msgs {
			if m.IsFromCust {
				lastCustAt = m.InsertedAt
			} else if lastCustAt > 0 {
				diffSec := m.InsertedAt - lastCustAt
				diffMin := float64(diffSec) / 60
				if v, ok := result[convId]; !ok || diffMin < v {
					result[convId] = diffMin
				}
				lastCustAt = 0
			}
		}
	}
	return result, nil
}

func (s *ReportService) loadConvertedCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, from, to time.Time) (map[string]bool, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return make(map[string]bool), nil
	}
	fromSec := from.Unix()
	toSec := to.Unix()
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"insertedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
					{"posCreatedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
				},
			},
			{
				"$or": []bson.M{
					{"posData.status": bson.M{"$in": []int{2, 3, 16}}},
					{"status": bson.M{"$in": []int{2, 3, 16}}},
				},
			},
		},
	}
	opts := options.Find().SetProjection(bson.M{"customerId": 1, "posData.customer.id": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			CustomerId string `bson:"customerId"`
			PosData    struct {
				Customer struct {
					ID string `bson:"id"`
				} `bson:"customer"`
			} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cid := doc.CustomerId
		if cid == "" {
			cid = doc.PosData.Customer.ID
		}
		if cid != "" {
			result[cid] = true
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return result, nil
}

// buildInboxConversationItem tạo InboxConversationItem từ inboxConvData.
func buildInboxConversationItem(c inboxConvData, pageName string, respMin float64) (reportdto.InboxConversationItem, bool, bool, int64, float64) {
	isBacklog := isLastSentByCustomer(c.PanCakeData)
	isUnassigned := isBacklog && isCurrentAssignEmpty(c.PanCakeData)
	waitingMins := int64(0)
	if isBacklog {
		waitingMins = (time.Now().Unix() - c.UpdatedAt) / 60
	}
	status := "replied"
	if isBacklog {
		status = "waiting"
	}
	lastMsgAt := ""
	if c.UpdatedAt > 0 {
		lastMsgAt = time.Unix(c.UpdatedAt, 0).Format("2006-01-02T15:04:05")
	}
	respMinOut := respMin
	if isBacklog {
		respMinOut = -1
	}
	return reportdto.InboxConversationItem{
		ConversationID:      c.ConversationId,
		PageID:              c.PageId,
		PageName:            pageName,
		CustomerName:        c.CustomerName,
		LastMessageAt:       lastMsgAt,
		LastMessageSnippet:  extractLastMessageSnippet(c.PanCakeData),
		Status:              status,
		WaitingMinutes:      waitingMins,
		ResponseTimeMin:     respMinOut,
		AssignedSale:        extractAssignedSaleName(c.PanCakeData),
		Tags:                extractTags(c.PanCakeData),
		IsBacklog:           isBacklog,
		IsUnassigned:        isUnassigned,
	}, isBacklog, isUnassigned, waitingMins, respMin
}

func filterInboxConversations(items []reportdto.InboxConversationItem, filter string) []reportdto.InboxConversationItem {
	switch filter {
	case "backlog":
		var out []reportdto.InboxConversationItem
		for _, it := range items {
			if it.IsBacklog {
				out = append(out, it)
			}
		}
		return out
	case "unassigned":
		var out []reportdto.InboxConversationItem
		for _, it := range items {
			if it.IsUnassigned {
				out = append(out, it)
			}
		}
		return out
	default:
		return items
	}
}

func sortInboxConversations(items []reportdto.InboxConversationItem, sortBy string) {
	switch sortBy {
	case "waiting_desc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].WaitingMinutes > items[j].WaitingMinutes
		})
	case "updated_asc":
		sort.Slice(items, func(i, j int) bool {
			return items[i].LastMessageAt < items[j].LastMessageAt
		})
	default:
		// updated_desc — mặc định
		sort.Slice(items, func(i, j int) bool {
			return items[i].LastMessageAt > items[j].LastMessageAt
		})
	}
}

func paginateInboxItems(items []reportdto.InboxConversationItem, offset, limit int) []reportdto.InboxConversationItem {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []reportdto.InboxConversationItem{}
	}
	toIdx := offset + limit
	if toIdx > len(items) {
		toIdx = len(items)
	}
	return items[offset:toIdx]
}

// buildSalePerformance tạo danh sách Sale Performance theo sale (current_assign_users, last_sent_by, tags NV).
func (s *ReportService) buildSalePerformance(convs []inboxConvData, responseTimes map[string]float64, convertedCustomers map[string]bool) []reportdto.InboxSalePerformanceItem {
	saleStats := make(map[string]*struct {
		Convs   int64
		RespSum float64
		RespCnt int
		Convert int64
	})
	for _, c := range convs {
		saleName := extractAssignedSaleName(c.PanCakeData)
		if saleName == "" {
			tags := extractTags(c.PanCakeData)
			for _, t := range tags {
				if strings.HasPrefix(t, "NV") || strings.HasPrefix(t, "nv") {
					saleName = t
					break
				}
			}
		}
		if saleName == "" {
			saleName = "Chưa assign"
		}
		if saleStats[saleName] == nil {
			saleStats[saleName] = &struct {
				Convs   int64
				RespSum float64
				RespCnt int
				Convert int64
			}{}
		}
		st := saleStats[saleName]
		st.Convs++
		resp := responseTimes[c.ConversationId]
		if resp >= 0 {
			st.RespSum += resp
			st.RespCnt++
		}
		if c.CustomerId != "" && convertedCustomers[c.CustomerId] {
			st.Convert++
		}
	}
	var result []reportdto.InboxSalePerformanceItem
	for name, st := range saleStats {
		medianResp := float64(0)
		if st.RespCnt > 0 {
			medianResp = st.RespSum / float64(st.RespCnt)
		}
		convRate := float64(0)
		if st.Convs > 0 {
			convRate = float64(st.Convert) / float64(st.Convs)
		}
		result = append(result, reportdto.InboxSalePerformanceItem{
			SaleName:            name,
			ConversationsHandled: st.Convs,
			MedianResponseMin:   medianResp,
			ConversionRate:     convRate,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConversationsHandled > result[j].ConversationsHandled
	})
	if result == nil {
		result = []reportdto.InboxSalePerformanceItem{}
	}
	return result
}

// buildInboxAlerts tạo CRITICAL và WARNING cho Alert zone.
func (s *ReportService) buildInboxAlerts(convs []inboxConvData, pageNames map[string]string) reportdto.InboxAlerts {
	var critical, warning []reportdto.InboxAlertItem
	for _, c := range convs {
		isBacklog := isLastSentByCustomer(c.PanCakeData)
		if !isBacklog {
			continue
		}
		waitingMins := (time.Now().Unix() - c.UpdatedAt) / 60
		isUnassigned := isCurrentAssignEmpty(c.PanCakeData)
		pageName := pageNames[c.PageId]
		if pageName == "" {
			pageName = c.PageId
		}
		item := reportdto.InboxAlertItem{
			ConversationID: c.ConversationId,
			CustomerName:   c.CustomerName,
			PageName:       pageName,
			WaitingMinutes: waitingMins,
			IsUnassigned:   isUnassigned,
		}
		if waitingMins > BacklogWaitingCriticalMinutes {
			critical = append(critical, item)
		} else {
			warning = append(warning, item)
		}
	}
	sort.Slice(critical, func(i, j int) bool {
		return critical[i].WaitingMinutes > critical[j].WaitingMinutes
	})
	sort.Slice(warning, func(i, j int) bool {
		return warning[i].WaitingMinutes > warning[j].WaitingMinutes
	})
	if len(critical) > 10 {
		critical = critical[:10]
	}
	if len(warning) > 10 {
		warning = warning[:10]
	}
	return reportdto.InboxAlerts{Critical: critical, Warning: warning}
}

func medianFloat64(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	sorted := make([]float64, len(a))
	copy(sorted, a)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func percentileFloat64(a []float64, p int) float64 {
	if len(a) == 0 {
		return 0
	}
	sorted := make([]float64, len(a))
	copy(sorted, a)
	sort.Float64s(sorted)
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
