// Package reportdto - DTO cho Inbox Operations (Tab 7 Dashboard).
// Kiểm soát xử lý lead: KPI, bảng hội thoại, Sale performance, Alert zone.
package reportdto

// InboxQueryParams query params cho GET /dashboard/inbox.
// Tab 7 dùng dữ liệu realtime/snapshot, không filter theo chu kỳ (trừ conversion).
type InboxQueryParams struct {
	PageID   string `query:"pageId"`   // Lọc theo page FB (optional)
	Filter   string `query:"filter"`   // backlog|unassigned|all|engaged — chip filter bảng hội thoại
	Limit    int    `query:"limit"`    // Số dòng conversation tối đa (mặc định 50)
	Offset   int    `query:"offset"`   // Phân trang
	Sort     string `query:"sort"`     // waiting_desc|updated_desc|updated_asc|care_priority
	Period   string `query:"period"`   // Chỉ dùng cho Conversion: day|week|month|60d|90d
	Engaged  bool   `query:"engaged"`  // true: chỉ hiện hội thoại từ khách Engaged (chưa mua)
}

// InboxSummary KPI cho Row 1 Tab 7 (6 KPI gốc + Engaged Intelligence).
type InboxSummary struct {
	ConversationsToday int64   `json:"conversationsToday"` // Hội thoại hôm nay (DATE(updated_at)=TODAY)
	BacklogCount       int64   `json:"backlogCount"`       // Backlog: tin cuối từ khách, chưa reply
	MedianResponseMin float64 `json:"medianResponseMin"`  // TB phản hồi (median) — phút
	P90ResponseMin    float64 `json:"p90ResponseMin"`     // P90 response time — phút
	UnassignedCount   int64   `json:"unassignedCount"`    // Chưa assign: backlog + current_assign_users rỗng
	ConversionRate    float64 `json:"conversionRate"`     // Hội thoại → đơn / Tổng trong period
	// Engaged Intelligence (Phase 1)
	EngagedCount    int64 `json:"engagedCount"`    // Số khách Engaged (đã chat, chưa mua)
	EngagedAging1d  int64 `json:"engagedAging1d"`  // Engaged kẹt > 1 ngày không tương tác
	EngagedAging3d  int64 `json:"engagedAging3d"`  // Engaged kẹt > 3 ngày
	EngagedAging7d  int64 `json:"engagedAging7d"`  // Engaged kẹt > 7 ngày
}

// EngagedMetrics metrics dành riêng cho stage Engaged (đã chat, chưa mua).
// Dùng để phân bổ nguồn lực chăm sóc.
type EngagedMetrics struct {
	Temperature     string `json:"temperature"`     // hot|warm|cooling|cold — nhiệt độ hiện tại
	EngagementDepth string `json:"engagementDepth"` // light|medium|deep — độ sâu tương tác
	SourceType      string `json:"sourceType"`      // organic|ads — nguồn hội thoại
	CarePriority    string `json:"carePriority"`    // P0|P1|P2|P3|P4 — mức ưu tiên chăm sóc
}

// InboxConversationItem 1 dòng trong bảng hội thoại Row 2.
type InboxConversationItem struct {
	ConversationID     string   `json:"conversationId"`
	PageID             string   `json:"pageId"`
	PageName           string   `json:"pageName"`
	CustomerID         string   `json:"customerId,omitempty"` // Để filter engaged (chưa mua)
	CustomerName       string   `json:"customerName"`
	LastMessageAt      string   `json:"lastMessageAt"`      // ISO format
	LastMessageSnippet string   `json:"lastMessageSnippet"`
	Status             string   `json:"status"`            // waiting|replied — để xác định màu row
	WaitingMinutes     int64    `json:"waitingMinutes"`    // Thời gian chờ (phút) — 0 nếu đã reply
	ResponseTimeMin    float64  `json:"responseTimeMin"`   // Thời gian phản hồi cuối (phút), -1 nếu chưa
	AssignedSale       string   `json:"assignedSale"`      // Tên sale assign
	Tags               []string `json:"tags"`              // Tags (NV.xx)
	IsBacklog          bool     `json:"isBacklog"`         // Tin cuối từ khách, chưa reply
	IsUnassigned       bool     `json:"isUnassigned"`      // Backlog + chưa assign
	// Engaged Intelligence (Phase 1) — nested theo stage
	Engaged   *EngagedMetrics `json:"engaged,omitempty"`   // Metrics stage Engaged (luôn có cho mỗi hội thoại)
	IsEngaged bool            `json:"isEngaged"`            // true: khách chưa mua (Engaged trong journey)
}

// InboxSalePerformanceItem 1 dòng trong bảng Sale Performance Row 3.
type InboxSalePerformanceItem struct {
	SaleName          string  `json:"saleName"`          // Từ current_assign_users hoặc last_sent_by / tag NV
	ConversationsHandled int64 `json:"conversationsHandled"` // Số hội thoại xử lý
	MedianResponseMin float64 `json:"medianResponseMin"` // TB response time
	ConversionRate    float64 `json:"conversionRate"`    // Hội thoại → đơn / Tổng
}

// InboxAlertItem mục trong Alert zone (CRITICAL/WARNING).
type InboxAlertItem struct {
	ConversationID   string `json:"conversationId"`
	CustomerName     string `json:"customerName"`
	PageName         string `json:"pageName"`
	WaitingMinutes   int64  `json:"waitingMinutes"`
	IsUnassigned     bool   `json:"isUnassigned"`
}

// InboxAlerts danh sách critical và warning.
type InboxAlerts struct {
	Critical []InboxAlertItem `json:"critical"`
	Warning  []InboxAlertItem `json:"warning"`
}

// InboxPageOption option cho Page filter (Row 0).
type InboxPageOption struct {
	PageID   string `json:"pageId"`
	PageName string `json:"pageName"`
}

// InboxSnapshotResult kết quả trả về cho GET /dashboard/inbox.
// Bao gồm: pages (filter), summary (6 KPI), conversations, salePerformance, alerts.
type InboxSnapshotResult struct {
	Pages            []InboxPageOption           `json:"pages"`            // Danh sách page để chọn filter
	Summary          InboxSummary                `json:"summary"`
	Conversations    []InboxConversationItem     `json:"conversations"`
	SalePerformance  []InboxSalePerformanceItem  `json:"salePerformance"`
	Alerts           InboxAlerts                 `json:"alerts"`
}
