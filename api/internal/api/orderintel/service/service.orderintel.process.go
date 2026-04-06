// Package orderintelsvc — Tích hợp AI Decision: hydrate order → tính snapshot → lưu → một event order_intel_recomputed.
package orderintelsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	ordermodels "meta_commerce/internal/api/order/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RunOrderIntelComputeJob tính Raw→L1→L2→L3→Flags từ job domain (gọi từ worker order_intel_compute, không gọi từ consumer AI Decision).
func RunOrderIntelComputeJob(ctx context.Context, job *orderintelmodels.OrderIntelComputeJob) error {
	if job == nil {
		return nil
	}
	ownerOrgID := job.OwnerOrganizationID
	now := time.Now().UnixMilli()
	view, err := loadOrderForJob(ctx, job)
	if err != nil {
		_, _ = persistOrderIntelAfterJob(ctx, job, nil, nil, orderintelmodels.OrderIntelRaw{EvaluatedAtMs: now}, err, now)
		return err
	}
	if view == nil {
		return nil
	}

	raw := BuildOrderIntelRaw(view, now)
	snap := ComputeSnapshot(view, now)
	if snap == nil {
		return nil
	}
	snap.Raw = raw
	snap.LastIntelRunId = primitive.NilObjectID

	prev, _ := findPreviousSnapshot(ctx, snap)

	if err := upsertSnapshot(ctx, snap); err != nil {
		return err
	}

	runID, perr := persistOrderIntelAfterJob(ctx, job, view, snap, raw, nil, now)
	if perr != nil {
		return perr
	}
	if !runID.IsZero() {
		if uerr := patchSnapshotLastIntelRunID(ctx, snap, runID); uerr != nil {
			logger.GetAppLogger().WithError(uerr).WithField("runId", runID.Hex()).Warn("📋 [ORDER_INTEL] Không gắn lastIntelRunId lên order_intelligence_snapshots")
		}
	}

	flagsChanged := prev == nil || !stringSliceEqual(prev.Flags, snap.Flags)
	completedNow := snap.Layer1.Stage == "completed"
	completedBefore := prev != nil && prev.Layer1.Stage == "completed"
	orderCompletedTransition := completedNow && !completedBefore

	flagIfaces := make([]interface{}, len(snap.Flags))
	for i, f := range snap.Flags {
		flagIfaces[i] = f
	}
	mongoHex := ""
	if !view.ID.IsZero() {
		mongoHex = view.ID.Hex()
	}
	pancakeHex := ""
	if !view.PancakeSourceMongoID.IsZero() {
		pancakeHex = view.PancakeSourceMongoID.Hex()
	}
	commerceHex := ""
	if !view.CommerceMongoID.IsZero() {
		commerceHex = view.CommerceMongoID.Hex()
	}
	extras := map[string]interface{}{
		"orderId":                  snap.OrderID,
		"flags":                    flagIfaces,
		"layer1":                   snap.Layer1,
		"layer2":                   snap.Layer2,
		"layer3":                   snap.Layer3,
		"flagsChanged":             flagsChanged,
		"orderCompletedTransition": orderCompletedTransition,
		"totalAfterDiscountVnd":    snap.Layer2.TotalAfterDiscountVND,
		"orderMongoId":             mongoHex,
		"commerceOrderMongoId":     commerceHex,
		"pancakeOrderMongoId":      pancakeHex,
		"sourceEventId":            job.ID.Hex(),
		"sourceEventType":          "order_intel.domain_job",
	}
	if !runID.IsZero() {
		extras["lastIntelRunId"] = runID.Hex()
	}

	return intelrecomputed.EmitOrderIntelRecomputed(ctx, ownerOrgID, job.ID.Hex(), snap.OrderUid, snap.Trace.CustomerID, snap.Trace.ConversationID, job.ParentEventID, job.ParentEventType, job.TraceID, job.CorrelationID, extras)
}

func loadOrderForJob(ctx context.Context, job *orderintelmodels.OrderIntelComputeJob) (*intelOrderView, error) {
	commerceColl, okCo := global.RegistryCollections.Get(global.MongoDB_ColNames.CommerceOrders)
	pcColl, okPc := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPc {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}
	ownerOrgID := job.OwnerOrganizationID

	tryCommerce := func() *intelOrderView {
		if !okCo || commerceColl == nil {
			return nil
		}
		if job.OrderUid != "" {
			var co ordermodels.CommerceOrder
			err := commerceColl.FindOne(ctx, bson.M{"uid": job.OrderUid, "ownerOrganizationId": ownerOrgID}).Decode(&co)
			if err == nil {
				return newIntelViewFromCommerce(&co)
			}
		}
		idHex := strings.TrimSpace(job.MongoRecordIdHex)
		if idHex == "" {
			idHex = strings.TrimSpace(job.NormalizedRecordUid)
		}
		if idHex == "" {
			return nil
		}
		oid, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			return nil
		}
		var co ordermodels.CommerceOrder
		err = commerceColl.FindOne(ctx, bson.M{
			"ownerOrganizationId": ownerOrgID,
			"source":              ordermodels.SourcePancakePOS,
			"sourceRecordMongoId": oid,
		}).Decode(&co)
		if err == nil {
			return newIntelViewFromCommerce(&co)
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		return nil
	}

	if v := tryCommerce(); v != nil {
		return v, nil
	}

	// Fallback: bản ghi Pancake chưa kịp chiếu lên commerce_orders (race hiếm / job cũ).
	if job.OrderUid != "" {
		var doc pcmodels.PcPosOrder
		err := pcColl.FindOne(ctx, bson.M{"uid": job.OrderUid, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, err
		}
		return newIntelViewFromPC(&doc), nil
	}
	idHex := strings.TrimSpace(job.MongoRecordIdHex)
	if idHex == "" {
		idHex = strings.TrimSpace(job.NormalizedRecordUid)
	}
	if idHex == "" {
		return nil, nil
	}
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, nil
	}
	var doc pcmodels.PcPosOrder
	err = pcColl.FindOne(ctx, bson.M{"_id": oid, "ownerOrganizationId": ownerOrgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return newIntelViewFromPC(&doc), nil
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

// snapshotUpsertFilter khóa Mongo theo contract: ưu tiên canonical orderUid (ord_*); nếu trống thì orderId POS + org (external, lớp 3).
func snapshotUpsertFilter(snap *orderintelmodels.OrderIntelligenceSnapshot) bson.M {
	if snap == nil || snap.OwnerOrganizationID.IsZero() {
		return nil
	}
	org := snap.OwnerOrganizationID
	if u := strings.TrimSpace(snap.OrderUid); u != "" {
		return bson.M{"orderUid": u, "ownerOrganizationId": org}
	}
	if snap.OrderID > 0 {
		return bson.M{"orderId": snap.OrderID, "ownerOrganizationId": org}
	}
	return nil
}

func findPreviousSnapshot(ctx context.Context, snap *orderintelmodels.OrderIntelligenceSnapshot) (*orderintelmodels.OrderIntelligenceSnapshot, error) {
	if snap == nil {
		return nil, nil
	}
	filter := snapshotUpsertFilter(snap)
	if filter == nil {
		return nil, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelligenceSnapshots)
	if !ok {
		return nil, nil
	}
	var doc orderintelmodels.OrderIntelligenceSnapshot
	err := coll.FindOne(ctx, filter).Decode(&doc)
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
	filter := snapshotUpsertFilter(snap)
	if filter == nil {
		return fmt.Errorf("snapshot order intelligence: thiếu khóa lưu (cần uid chuẩn ord_* hoặc orderId POS)")
	}
	now := snap.UpdatedAt
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	setDoc := bson.M{
		"orderId":   snap.OrderID,
		"raw":       snap.Raw,
		"layer1":    snap.Layer1,
		"layer2":    snap.Layer2,
		"layer3":    snap.Layer3,
		"flags":     snap.Flags,
		"trace":     snap.Trace,
		"updatedAt": now,
	}
	if u := strings.TrimSpace(snap.OrderUid); u != "" {
		setDoc["orderUid"] = u
	} else {
		setDoc["orderUid"] = ""
	}
	if !snap.LastIntelRunId.IsZero() {
		setDoc["lastIntelRunId"] = snap.LastIntelRunId
	}
	update := bson.M{
		"$set":         setDoc,
		"$setOnInsert": bson.M{"createdAt": now},
	}
	if snap.LastIntelRunId.IsZero() {
		update["$unset"] = bson.M{"lastIntelRunId": ""}
	}
	_, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// patchSnapshotLastIntelRunID gắn pointer lớp A lên read model B sau khi insert order_intel_runs.
func patchSnapshotLastIntelRunID(ctx context.Context, snap *orderintelmodels.OrderIntelligenceSnapshot, runID primitive.ObjectID) error {
	if snap == nil || runID.IsZero() {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelligenceSnapshots)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.OrderIntelligenceSnapshots, common.ErrNotFound)
	}
	filter := snapshotUpsertFilter(snap)
	if filter == nil {
		return fmt.Errorf("snapshot order intelligence: thiếu khóa lưu")
	}
	_, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"lastIntelRunId": runID}})
	return err
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
