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
	registryMutex sync.RWMutex
)

// RegisterExecutor đăng ký executor cho domain.
func (e *Engine) RegisterExecutor(domain string, ex Executor) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	executors[domain] = ex
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
	doc.Status = StatusApproved
	doc.ApprovedAt = now
	doc.UpdatedAt = now

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

func (e *Engine) getEventType(domain, event string) string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	if m, ok := eventTypes[domain]; ok {
		return m[event]
	}
	return ""
}
