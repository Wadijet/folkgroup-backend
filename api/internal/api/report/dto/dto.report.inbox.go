// Package reportdto - DTO cho Inbox Operations (Tab 7 Dashboard).
// Kiểm soát xử lý lead: KPI, bảng hội thoại, Sale performance, Alert zone.
package reportdto

// InboxQueryParams query params cho GET /dashboard/inbox.
// Tab 7 dùng dữ liệu realtime/snapshot, không filter theo chu kỳ (trừ conversion).
type InboxQueryParams struct {
	PageID string `query:"pageId"`   // Lọc theo page FB (optional)
	Filter string `query:"filter"`   // backlog|unassigned|all — chip filter bảng hội thoại
	Limit  int    `query:"limit"`    // Số dòng conversation tối đa (mặc định 50)
	Offset int    `query:"offset"`   // Phân trang
	Sort   string `query:"sort"`     // waiting_desc|updated_desc|updated_asc
	Period string `query:"period"`  // Chỉ dùng cho Conversion: day|week|month|60d|90d
}

// InboxSummary 6 KPI cho Row 1 Tab 7.
type InboxSummary struct {
	ConversationsToday int64   `json:"conversationsToday"` // Hội thoại hôm nay (DATE(updated_at)=TODAY)
	BacklogCount       int64   `json:"backlogCount"`       // Backlog: tin cuối từ khách, chưa reply
	MedianResponseMin  float64 `json:"medianResponseMin"`  // TB phản hồi (median) — phút
	P90ResponseMin     float64 `json:"p90ResponseMin"`     // P90 response time — phút
	UnassignedCount    int64   `json:"unassignedCount"`   // Chưa assign: backlog + current_assign_users rỗng
	ConversionRate     float64 `json:"conversionRate"`     // Hội thoại → đơn / Tổng trong period
}

// InboxConversationItem 1 dòng trong bảng hội thoại Row 2.
type InboxConversationItem struct {
	ConversationID  string   `json:"conversationId"`
	PageID          string   `json:"pageId"`
	PageName        string   `json:"pageName"`
	CustomerName    string   `json:"customerName"`
	LastMessageAt   string   `json:"lastMessageAt"`   // ISO format
	LastMessageSnippet string `json:"lastMessageSnippet"`
	Status          string   `json:"status"`          // waiting|replied — để xác định màu row
	WaitingMinutes   int64   `json:"waitingMinutes"`   // Thời gian chờ (phút) — 0 nếu đã reply
	ResponseTimeMin  float64 `json:"responseTimeMin"`  // Thời gian phản hồi cuối (phút), -1 nếu chưa
	AssignedSale     string   `json:"assignedSale"`    // Tên sale assign
	Tags             []string `json:"tags"`            // Tags (NV.xx)
	IsBacklog        bool    `json:"isBacklog"`        // Tin cuối từ khách, chưa reply
	IsUnassigned     bool    `json:"isUnassigned"`     // Backlog + chưa assign
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
