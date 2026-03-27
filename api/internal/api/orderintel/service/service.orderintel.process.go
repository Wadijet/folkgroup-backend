// Package orderintelsvc — Tích hợp AI Decision: hydrate order → tính snapshot → lưu → emit event.
package orderintelsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventTypeOrderFlagsEmitted — đồng bộ consumer AI Decision (order.flags_emitted).
const EventTypeOrderFlagsEmitted = "order.flags_emitted"

// EventTypeCommerceOrderCompleted — Learning Engine / pipeline sau đơn hoàn thành (Vision §3.4).
const EventTypeCommerceOrderCompleted = "commerce.order_completed"

// OrderIntelEmitContext ngữ cảnh báo cáo kết quả về decision_events_queue (sau khi tính tại domain worker).
type OrderIntelEmitContext struct {
	TraceID         string
	CorrelationID   string
	OrgID           string
	OwnerOrgID      primitive.ObjectID
	ParentEventID   string
	ParentEventType string
	DomainJobIDHex  string // id job order_intelligence_pending — trace domain
}

// RunPendingJob tính Raw→L1→L2→L3→Flags từ job domain (gọi từ Order Intelligence worker, không gọi từ consumer AI Decision).
func RunPendingJob(ctx context.Context, job *orderintelmodels.OrderIntelligencePendingJob) error {
	if job == nil {
		return nil
	}
	ownerOrgID := job.OwnerOrganizationID
	order, err := loadOrderForJob(ctx, job)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}

	now := time.Now().UnixMilli()
	snap := ComputeSnapshot(order, now)
	if snap == nil {
		return nil
	}

	prev, _ := findPreviousSnapshot(ctx, snap.OrderUid, ownerOrgID)

	if err := upsertSnapshot(ctx, snap); err != nil {
		return err
	}

	emitCtx := &OrderIntelEmitContext{
		TraceID:         job.TraceID,
		CorrelationID:   job.CorrelationID,
		OrgID:           job.OrgID,
		OwnerOrgID:      ownerOrgID,
		ParentEventID:   job.ParentEventID,
		ParentEventType: job.ParentEventType,
		DomainJobIDHex:  job.ID.Hex(),
	}
	decSvc := aidecisionsvc.NewAIDecisionService()
	return emitFollowUpEvents(ctx, decSvc, emitCtx, snap, prev)
}

func loadOrderForJob(ctx context.Context, job *orderintelmodels.OrderIntelligencePendingJob) (*pcmodels.PcPosOrder, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}
	ownerOrgID := job.OwnerOrganizationID
	if job.OrderUid != "" {
		var doc pcmodels.PcPosOrder
		err := coll.FindOne(ctx, bson.M{"uid": job.OrderUid, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, err
		}
		return &doc, nil
	}
	idHex := strings.TrimSpace(job.MongoRecordIdHex)
	if idHex == "" {
		return nil, nil
	}
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, nil
	}
	var doc pcmodels.PcPosOrder
	err = coll.FindOne(ctx, bson.M{"_id": oid, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func strFromPayload(p map[string]interface{}, key string) string {
	v, ok := p[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", t))
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

// normalizeOrderIntelligencePayload đồng bộ payload order.recompute_requested với order.intelligence_requested (orderId → orderUid).
func normalizeOrderIntelligencePayload(evt *aidecisionmodels.DecisionEvent) {
	if evt.Payload == nil {
		return
	}
	p := evt.Payload
	if strFromPayload(p, "orderUid") == "" {
		if s := strFromPayload(p, "orderId"); s != "" {
			p["orderUid"] = s
		}
	}
}

func findPreviousSnapshot(ctx context.Context, orderUid string, ownerOrgID primitive.ObjectID) (*orderintelmodels.OrderIntelligenceSnapshot, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelligenceSnapshots)
	if !ok || orderUid == "" {
		return nil, nil
	}
	var doc orderintelmodels.OrderIntelligenceSnapshot
	err := coll.FindOne(ctx, bson.M{"orderUid": orderUid, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func upsertSnapshot(ctx context.Context, snap *orderintelmodels.OrderIntelligenceSnapshot) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelligenceSnapshots)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.OrderIntelligenceSnapshots, common.ErrNotFound)
	}
	now := snap.UpdatedAt
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	filter := bson.M{"orderUid": snap.OrderUid, "ownerOrganizationId": snap.OwnerOrganizationID}
	setDoc := bson.M{
		"orderId":             snap.OrderID,
		"layer1":              snap.Layer1,
		"layer2":              snap.Layer2,
		"layer3":              snap.Layer3,
		"flags":               snap.Flags,
		"trace":               snap.Trace,
		"updatedAt":           now,
	}
	update := bson.M{
		"$set":         setDoc,
		"$setOnInsert": bson.M{"createdAt": now},
	}
	_, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func emitFollowUpEvents(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, emitCtx *OrderIntelEmitContext, snap *orderintelmodels.OrderIntelligenceSnapshot, prev *orderintelmodels.OrderIntelligenceSnapshot) error {
	if emitCtx == nil || snap == nil {
		return nil
	}
	ownerOrgID := emitCtx.OwnerOrgID
	if ownerOrgID.IsZero() {
		ownerOrgID = snap.OwnerOrganizationID
	}
	orgID := emitCtx.OrgID
	if orgID == "" {
		orgID = ownerOrgID.Hex()
	}

	flagsChanged := prev == nil || !stringSliceEqual(prev.Flags, snap.Flags)
	if len(snap.Flags) > 0 && flagsChanged {
		flagIfaces := make([]interface{}, len(snap.Flags))
		for i, f := range snap.Flags {
			flagIfaces[i] = f
		}
		payload := map[string]interface{}{
			"orderId":         snap.OrderUid,
			"customerId":      snap.Trace.CustomerID,
			"conversationId":  snap.Trace.ConversationID,
			"flags":           flagIfaces,
			"layer1":          snap.Layer1,
			"layer2":          snap.Layer2,
			"layer3":          snap.Layer3,
			"ownerOrgIdHex":   ownerOrgID.Hex(),
			"sourceEventId":   emitCtx.DomainJobIDHex,
			"sourceEventType": "order_intel.domain_job",
		}
		if emitCtx.ParentEventID != "" {
			payload["parentEventId"] = emitCtx.ParentEventID
			payload["parentEventType"] = emitCtx.ParentEventType
		}
		_, err := decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
			EventType:     EventTypeOrderFlagsEmitted,
			EventSource:   "orderintel",
			EntityType:    "order",
			EntityID:      snap.OrderUid,
			OrgID:         orgID,
			OwnerOrgID:    ownerOrgID,
			Priority:      "high",
			Lane:          aidecisionmodels.EventLaneFast,
			TraceID:       emitCtx.TraceID,
			CorrelationID: emitCtx.CorrelationID,
			Payload:       payload,
		})
		if err != nil {
			return err
		}
	}

	completedNow := snap.Layer1.Stage == "completed"
	completedBefore := prev != nil && prev.Layer1.Stage == "completed"
	if completedNow && !completedBefore {
		_, err := decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
			EventType:     EventTypeCommerceOrderCompleted,
			EventSource:   "orderintel",
			EntityType:    "order",
			EntityID:      snap.OrderUid,
			OrgID:         orgID,
			OwnerOrgID:    ownerOrgID,
			Priority:      "normal",
			Lane:          aidecisionmodels.EventLaneFast,
			TraceID:       emitCtx.TraceID,
			CorrelationID: emitCtx.CorrelationID,
			Payload: map[string]interface{}{
				"orderUid":              snap.OrderUid,
				"orderId":               snap.OrderID,
				"customerId":            snap.Trace.CustomerID,
				"conversationId":      snap.Trace.ConversationID,
				"ownerOrgIdHex":         ownerOrgID.Hex(),
				"layer1":                snap.Layer1,
				"totalAfterDiscountVnd": snap.Layer2.TotalAfterDiscountVND,
				"flags":                 snap.Flags,
				"sourceEventId":         emitCtx.DomainJobIDHex,
				"sourceEventType":       "order_intel.domain_job",
				"parentEventId":         emitCtx.ParentEventID,
				"parentEventType":       emitCtx.ParentEventType,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
