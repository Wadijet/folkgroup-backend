// Package orderintelsvc — Lớp A order_intel_runs + pointer intel trên order_canonical (chuẩn hai lớp A/B).
package orderintelsvc

import (
	"context"
	"strings"
	"time"

	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func causalForOrderIntelMs(job *orderintelmodels.OrderIntelComputeJob) int64 {
	if job == nil {
		return time.Now().UnixMilli()
	}
	if job.CausalOrderingAtMs > 0 {
		return job.CausalOrderingAtMs
	}
	return time.Now().UnixMilli()
}

func buildOrderIntelSummary(snap *orderintelmodels.OrderIntelligenceSnapshot) bson.M {
	if snap == nil {
		return nil
	}
	return bson.M{
		"layer1": snap.Layer1,
		"layer2": snap.Layer2,
		"layer3": snap.Layer3,
		"flags":  snap.Flags,
	}
}

// persistOrderIntelAfterJob ghi một bản ghi order_intel_runs; thành công và có order_canonical thì $inc intelSequence + cập nhật pointer.
// Thứ tự sort lịch sử đề xuất: causalOrderingAt tăng, intelSequence tăng, _id.
func persistOrderIntelAfterJob(ctx context.Context, job *orderintelmodels.OrderIntelComputeJob, view *intelOrderView, snap *orderintelmodels.OrderIntelligenceSnapshot, raw orderintelmodels.OrderIntelRaw, execErr error, nowMs int64) (primitive.ObjectID, error) {
	if job == nil {
		return primitive.NilObjectID, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelRuns)
	if !ok || coll == nil {
		logger.GetAppLogger().Warn("📋 [ORDER_INTEL_RUN] Collection order_intel_runs chưa đăng ký — bỏ qua persist")
		return primitive.NilObjectID, nil
	}

	runID := primitive.NewObjectID()
	orderingAt := causalForOrderIntelMs(job)

	status := orderintelmodels.OrderIntelRunStatusSuccess
	errMsg := ""
	if execErr != nil {
		status = orderintelmodels.OrderIntelRunStatusFailed
		errMsg = execErr.Error()
	}

	orderUid := ""
	orderID := int64(0)
	canonicalOID := primitive.NilObjectID
	if snap != nil {
		orderUid = strings.TrimSpace(snap.OrderUid)
		orderID = snap.OrderID
	}
	if view != nil {
		if canonicalOID.IsZero() && !view.OrderCanonicalMongoID.IsZero() {
			canonicalOID = view.OrderCanonicalMongoID
		}
		if orderUid == "" {
			orderUid = strings.TrimSpace(view.Uid)
		}
		if orderID == 0 {
			orderID = view.OrderId
		}
	}
	if orderUid == "" {
		orderUid = strings.TrimSpace(job.OrderUid)
	}

	op := job.Source
	if op == "" {
		op = "order_intel_compute"
	}

	doc := orderintelmodels.OrderIntelRun{
		ID:                   runID,
		OwnerOrganizationID:  job.OwnerOrganizationID,
		OrderUid:             orderUid,
		OrderID:              orderID,
		OrderCanonicalMongoID: canonicalOID,
		Operation:            op,
		Status:               status,
		ParentIntelJobID:     job.ID,
		ParentEventID:        job.ParentEventID,
		ParentEventType:      job.ParentEventType,
		TraceID:              job.TraceID,
		CorrelationID:        job.CorrelationID,
		ComputedAt:           nowMs,
		CausalOrderingAt:     orderingAt,
		IntelSequence:        0,
		ErrorMessage:         errMsg,
		Raw:                  raw,
	}

	if execErr == nil && snap != nil {
		doc.IntelSummary = buildOrderIntelSummary(snap)
	}
	doc.IntelSequence = 0

	_, insErr := coll.InsertOne(ctx, doc)
	if insErr != nil && mongo.IsDuplicateKeyError(insErr) {
		var existing orderintelmodels.OrderIntelRun
		if err := coll.FindOne(ctx, bson.M{
			"parentIntelJobId":    job.ID,
			"ownerOrganizationId": job.OwnerOrganizationID,
		}).Decode(&existing); err == nil {
			return existing.ID, nil
		}
		logger.GetAppLogger().WithError(insErr).Warn("📋 [ORDER_INTEL_RUN] Trùng khóa job nhưng không đọc lại được run")
		return primitive.NilObjectID, insErr
	}
	if insErr != nil {
		logger.GetAppLogger().WithError(insErr).Warn("📋 [ORDER_INTEL_RUN] Insert run thất bại")
		return primitive.NilObjectID, insErr
	}

	if execErr != nil || canonicalOID.IsZero() || job.OwnerOrganizationID.IsZero() {
		return runID, nil
	}

	canonicalColl, okCo := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical)
	if !okCo || canonicalColl == nil {
		return runID, nil
	}

	var seq int64
	var bumped struct {
		IntelSequence int64 `bson:"intelSequence"`
	}
	incErr := canonicalColl.FindOneAndUpdate(ctx,
		bson.M{"_id": canonicalOID, "ownerOrganizationId": job.OwnerOrganizationID},
		bson.M{"$inc": bson.M{"intelSequence": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&bumped)
	if incErr != nil {
		logger.GetAppLogger().WithError(incErr).WithField("orderCanonicalId", canonicalOID.Hex()).Warn("📋 [ORDER_INTEL_RUN] Không $inc intelSequence trên order_canonical")
	} else {
		seq = bumped.IntelSequence
		_, _ = coll.UpdateOne(ctx, bson.M{"_id": runID}, bson.M{"$set": bson.M{"intelSequence": seq}})
	}

	_, uerr := canonicalColl.UpdateOne(ctx,
		bson.M{"_id": canonicalOID, "ownerOrganizationId": job.OwnerOrganizationID},
		bson.M{"$set": bson.M{
			"intelLastRunId":      runID,
			"intelLastComputedAt": nowMs,
			"updatedAt":           nowMs,
		}},
	)
	if uerr != nil {
		logger.GetAppLogger().WithError(uerr).WithField("orderCanonicalId", canonicalOID.Hex()).Warn("📋 [ORDER_INTEL_RUN] Không cập nhật pointer intelLastRunId trên order_canonical")
	}
	return runID, nil
}
