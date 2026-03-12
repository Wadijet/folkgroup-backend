package approval

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Engine cơ chế duyệt. Inject Storage + Notifier.
type Engine struct {
	storage  Storage
	notifier Notifier
}

// NewEngine tạo engine. App gọi với implementation của Storage, Notifier.
func NewEngine(storage Storage, notifier Notifier) *Engine {
	return &Engine{storage: storage, notifier: notifier}
}

var (
	executors     = make(map[string]Executor)
	eventTypes    = make(map[string]map[string]string) // domain -> event -> eventType
	deferredDomains = make(map[string]bool)           // domain dùng queue thay vì execute ngay
	registryMutex sync.RWMutex
)

// RegisterExecutor đăng ký executor cho domain.
func (e *Engine) RegisterExecutor(domain string, ex Executor) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	executors[domain] = ex
}

// RegisterDeferredExecutionDomain đăng ký domain dùng queue: sau khi approve, set status=queued thay vì execute ngay.
// Worker sẽ poll và xử lý với retry.
func RegisterDeferredExecutionDomain(domain string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	deferredDomains[domain] = true
}

func isDeferredExecutionDomain(domain string) bool {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return deferredDomains[domain]
}

// RegisterEventTypes đăng ký EventType cho domain (executed, rejected).
func (e *Engine) RegisterEventTypes(domain string, types map[string]string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	eventTypes[domain] = types
}

// ProposeInput input cho Propose.
type ProposeInput struct {
	ActionType       string
	Reason           string
	Payload          map[string]interface{}
	EventTypePending string
	ApprovePath      string
	RejectPath       string
}

// Propose thêm đề xuất vào queue.
func (e *Engine) Propose(ctx context.Context, domain string, input ProposeInput, ownerOrgID primitive.ObjectID, baseURL string) (*ActionPending, error) {
	now := time.Now().UnixMilli()
	doc := &ActionPending{
		Domain:              domain,
		ActionType:          input.ActionType,
		Reason:              input.Reason,
		Payload:             input.Payload,
		ProposedAt:          now,
		Status:              StatusPending,
		OwnerOrganizationID: ownerOrgID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := e.storage.Insert(ctx, doc); err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	eventType := input.EventTypePending
	if eventType == "" {
		eventType = "approval_pending_" + domain
	}
	approvePath := input.ApprovePath
	if approvePath == "" {
		approvePath = "/api/v1/approval/actions/approve"
	}
	rejectPath := input.RejectPath
	if rejectPath == "" {
		rejectPath = "/api/v1/approval/actions/reject"
	}
	payload := map[string]interface{}{
		"actionId":   doc.ID.Hex(),
		"domain":     domain,
		"actionType": doc.ActionType,
		"reason":     doc.Reason,
		"proposedAt": doc.ProposedAt,
		"approveUrl": baseURL + approvePath,
		"rejectUrl":  baseURL + rejectPath,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	for k, v := range doc.Payload {
		payload[k] = v
	}
	_, _ = e.notifier.Notify(ctx, eventType, payload, ownerOrgID, baseURL)
	return doc, nil
}

// Approve duyệt đề xuất.
func (e *Engine) Approve(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*ActionPending, error) {
	oid, err := primitive.ObjectIDFromHex(actionId)
	if err != nil {
		return nil, fmt.Errorf("actionId không hợp lệ")
	}
	doc, err := e.storage.FindById(ctx, oid, ownerOrgID)
	if err != nil {
		return nil, err
	}
	if doc.Status != StatusPending {
		return nil, fmt.Errorf("đề xuất không còn pending: %s", doc.Status)
	}
	now := time.Now().UnixMilli()
	doc.ApprovedAt = now
	doc.UpdatedAt = now

	// Domain dùng queue: set status=queued, worker sẽ xử lý với retry
	if isDeferredExecutionDomain(doc.Domain) {
		doc.Status = StatusQueued
		doc.RetryCount = 0
		doc.MaxRetries = MaxRetriesDefault
		doc.NextRetryAt = nil // Worker lấy ngay
		if err := e.storage.Update(ctx, doc); err != nil {
			return nil, err
		}
		return doc, nil
	}

	// Execute ngay (domain không dùng queue)
	doc.Status = StatusApproved
	registryMutex.RLock()
	ex := executors[doc.Domain]
	registryMutex.RUnlock()

	if ex != nil {
		resp, execErr := ex.Execute(ctx, doc)
		if execErr != nil {
			doc.Status = StatusFailed
			doc.ExecuteError = execErr.Error()
			doc.ExecuteResponse = map[string]interface{}{"error": execErr.Error()}
		} else {
			doc.Status = StatusExecuted
			doc.ExecuteResponse = resp
		}
	} else {
		doc.Status = StatusExecuted
		doc.ExecuteResponse = map[string]interface{}{"stub": true, "message": "Chưa đăng ký executor: " + doc.Domain}
	}
	doc.ExecutedAt = now

	if err := e.storage.Update(ctx, doc); err != nil {
		return nil, err
	}
	if et := e.getEventType(doc.Domain, "executed"); et != "" {
		p := map[string]interface{}{
			"actionId": doc.ID.Hex(), "actionType": doc.ActionType,
			"executedAt": doc.ExecutedAt, "executeResponse": doc.ExecuteResponse,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		for k, v := range doc.Payload {
			p[k] = v
		}
		_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
	}
	return doc, nil
}

// Reject từ chối đề xuất.
func (e *Engine) Reject(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID, decisionNote, rejectedBy string) (*ActionPending, error) {
	oid, err := primitive.ObjectIDFromHex(actionId)
	if err != nil {
		return nil, fmt.Errorf("actionId không hợp lệ")
	}
	doc, err := e.storage.FindById(ctx, oid, ownerOrgID)
	if err != nil {
		return nil, err
	}
	if doc.Status != StatusPending {
		return nil, fmt.Errorf("đề xuất không còn pending: %s", doc.Status)
	}
	now := time.Now().UnixMilli()
	doc.Status = StatusRejected
	doc.RejectedAt = now
	doc.RejectedBy = rejectedBy
	doc.DecisionNote = decisionNote
	doc.UpdatedAt = now
	if err := e.storage.Update(ctx, doc); err != nil {
		return nil, err
	}
	if et := e.getEventType(doc.Domain, "rejected"); et != "" {
		p := map[string]interface{}{
			"actionId": doc.ID.Hex(), "actionType": doc.ActionType,
			"reason": doc.DecisionNote, "rejectedAt": doc.RejectedAt, "rejectedBy": doc.RejectedBy,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		for k, v := range doc.Payload {
			p[k] = v
		}
		_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
	}
	return doc, nil
}

// ListPending danh sách đề xuất chờ duyệt.
func (e *Engine) ListPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]ActionPending, error) {
	return e.storage.FindPending(ctx, ownerOrgID, domain, limit)
}

// FindById xem chi tiết một đề xuất theo id — phục vụ frontend.
func (e *Engine) FindById(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*ActionPending, error) {
	oid, err := primitive.ObjectIDFromHex(actionId)
	if err != nil {
		return nil, fmt.Errorf("actionId không hợp lệ")
	}
	return e.storage.FindById(ctx, oid, ownerOrgID)
}

// Find danh sách với filter (domain, status, limit, sort) — phục vụ frontend xem.
func (e *Engine) Find(ctx context.Context, ownerOrgID primitive.ObjectID, filter FindFilter) ([]ActionPending, error) {
	return e.storage.Find(ctx, ownerOrgID, filter)
}

// FindWithPagination danh sách có phân trang — phục vụ frontend table.
func (e *Engine) FindWithPagination(ctx context.Context, ownerOrgID primitive.ObjectID, filter FindWithPaginationFilter) ([]ActionPending, int64, error) {
	return e.storage.FindWithPagination(ctx, ownerOrgID, filter)
}

// Count đếm theo filter — phục vụ dashboard badges.
func (e *Engine) Count(ctx context.Context, ownerOrgID primitive.ObjectID, domain, status string, fromProposedAt, toProposedAt int64) (int64, error) {
	return e.storage.Count(ctx, ownerOrgID, domain, status, fromProposedAt, toProposedAt)
}

// Cancel hủy đề xuất pending — chỉ cho phép khi status=pending.
func (e *Engine) Cancel(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*ActionPending, error) {
	oid, err := primitive.ObjectIDFromHex(actionId)
	if err != nil {
		return nil, fmt.Errorf("actionId không hợp lệ")
	}
	doc, err := e.storage.FindById(ctx, oid, ownerOrgID)
	if err != nil {
		return nil, err
	}
	if doc.Status != StatusPending {
		return nil, fmt.Errorf("chỉ có thể hủy đề xuất đang chờ duyệt (status=pending), hiện tại: %s", doc.Status)
	}
	now := time.Now().UnixMilli()
	doc.Status = StatusCancelled
	doc.UpdatedAt = now
	if err := e.storage.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("cập nhật: %w", err)
	}
	if et := e.getEventType(doc.Domain, "cancelled"); et != "" {
		p := map[string]interface{}{
			"actionId": doc.ID.Hex(), "actionType": doc.ActionType,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		for k, v := range doc.Payload {
			p[k] = v
		}
		_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
	}
	return doc, nil
}

// FindQueued danh sách item status=queued để worker xử lý (domain dùng deferred execution).
func (e *Engine) FindQueued(ctx context.Context, domain string, limit int) ([]ActionPending, error) {
	return e.storage.FindQueued(ctx, domain, limit)
}

// Update cập nhật document (worker dùng sau khi execute/retry).
func (e *Engine) Update(ctx context.Context, doc *ActionPending) error {
	return e.storage.Update(ctx, doc)
}

// NotifyExecuted gửi thông báo executed (worker gọi sau khi thực thi thành công).
func (e *Engine) NotifyExecuted(ctx context.Context, doc *ActionPending) {
	if et := e.getEventType(doc.Domain, "executed"); et != "" {
		p := map[string]interface{}{
			"actionId": doc.ID.Hex(), "actionType": doc.ActionType,
			"executedAt": doc.ExecutedAt, "executeResponse": doc.ExecuteResponse,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		for k, v := range doc.Payload {
			p[k] = v
		}
		_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
	}
}

// NotifyFailed gửi thông báo khi thực thi thất bại sau hết retry (worker gọi).
func (e *Engine) NotifyFailed(ctx context.Context, doc *ActionPending) {
	if et := e.getEventType(doc.Domain, "failed"); et != "" {
		p := map[string]interface{}{
			"actionId":      doc.ID.Hex(), "actionType": doc.ActionType,
			"executedAt":   doc.ExecutedAt, "executeError": doc.ExecuteError,
			"executeResponse": doc.ExecuteResponse, "retryCount": doc.RetryCount,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		for k, v := range doc.Payload {
			p[k] = v
		}
		_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
	}
}

// ExecuteOne thực thi thủ công một đề xuất đã duyệt (status=queued).
// Dùng cho test — user trigger thay vì chờ worker. Sau này hệ thống tự động qua worker.
func (e *Engine) ExecuteOne(ctx context.Context, actionId string, ownerOrgID primitive.ObjectID) (*ActionPending, error) {
	oid, err := primitive.ObjectIDFromHex(actionId)
	if err != nil {
		return nil, fmt.Errorf("actionId không hợp lệ")
	}
	doc, err := e.storage.FindById(ctx, oid, ownerOrgID)
	if err != nil {
		return nil, err
	}
	if doc.Status != StatusQueued {
		return nil, fmt.Errorf("chỉ có thể thực thi đề xuất đã duyệt (status=queued), hiện tại: %s", doc.Status)
	}
	registryMutex.RLock()
	ex := executors[doc.Domain]
	registryMutex.RUnlock()
	if ex == nil {
		return nil, fmt.Errorf("domain %s chưa đăng ký executor", doc.Domain)
	}
	now := time.Now().UnixMilli()
	doc.UpdatedAt = now
	resp, execErr := ex.Execute(ctx, doc)
	if execErr == nil {
		doc.Status = StatusExecuted
		doc.ExecuteResponse = resp
		doc.ExecutedAt = now
		doc.ExecuteError = ""
		doc.NextRetryAt = nil
		if err := e.storage.Update(ctx, doc); err != nil {
			return nil, fmt.Errorf("cập nhật kết quả: %w", err)
		}
		if et := e.getEventType(doc.Domain, "executed"); et != "" {
			p := map[string]interface{}{
				"actionId": doc.ID.Hex(), "actionType": doc.ActionType,
				"executedAt": doc.ExecutedAt, "executeResponse": doc.ExecuteResponse,
			}
			for k, v := range doc.Payload {
				p[k] = v
			}
			_, _ = e.notifier.Notify(ctx, et, p, doc.OwnerOrganizationID, "")
		}
		return doc, nil
	}
	// Thất bại: cập nhật và trả lỗi
	doc.Status = StatusFailed
	doc.ExecuteError = execErr.Error()
	doc.ExecuteResponse = map[string]interface{}{"error": execErr.Error()}
	doc.ExecutedAt = now
	doc.NextRetryAt = nil
	_ = e.storage.Update(ctx, doc)
	return nil, fmt.Errorf("thực thi thất bại: %w", execErr)
}

func (e *Engine) getEventType(domain, event string) string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	if m, ok := eventTypes[domain]; ok {
		return m[event]
	}
	return ""
}
