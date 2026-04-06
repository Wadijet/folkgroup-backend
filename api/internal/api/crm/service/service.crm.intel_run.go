// Package crmvc — Lưu lịch sử mỗi lần chạy intel khách (crm_customer_intel_runs) + pointer trên crm_customers.
package crmvc

import (
	"context"
	"strings"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	crmIntelRunStatusSuccess = "success"
	crmIntelRunStatusFailed  = "failed"
)

// crmIntelBatchStats — thống kê job intel đa khách / toàn org (không cập nhật intelLastRunId từng khách).
type crmIntelBatchStats struct {
	multi              bool
	processed          int
	failed             int
	orgCount           int
	classificationMode string
}

func payloadStrCRM(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func buildCrmCustomerMetricsSummary(c *crmmodels.CrmCustomer) bson.M {
	if c == nil {
		return nil
	}
	return bson.M{
		"unifiedId":         c.UnifiedId,
		"uid":               c.Uid,
		"valueTier":         c.ValueTier,
		"lifecycleStage":    c.LifecycleStage,
		"journeyStage":      c.JourneyStage,
		"momentumStage":     c.MomentumStage,
		"loyaltyStage":      c.LoyaltyStage,
		"channel":           c.Channel,
		"totalSpent":        c.TotalSpent,
		"orderCount":        c.OrderCount,
		"lastOrderAt":       c.LastOrderAt,
		"hasConversation":   c.HasConversation,
		"hasOrder":          c.HasOrder,
		"intelSequence":     c.IntelSequence,
	}
}

// causalForOrderingMs — thời điểm nghiệp vụ cho sort lịch sử; không có trong payload thì fallback wall-clock (yếu hơn nhưng vẫn tổng thể tăng).
func causalForOrderingMs(job *crmmodels.CrmIntelComputeJob) int64 {
	if job == nil || job.Payload == nil {
		return time.Now().UnixMilli()
	}
	v := crmqueue.ExtractCausalOrderingAtMs(job.Payload)
	if v <= 0 {
		return time.Now().UnixMilli()
	}
	return v
}

// persistCrmCustomerIntelAfterJob ghi CrmCustomerIntelRun và (nếu thành công + một khách) cập nhật intelLastRunId + intelSequence trên crm_customers.
// Thứ tự đọc lịch sử đề xuất: causalOrderingAt tăng dần, intelSequence tăng dần, _id.
func persistCrmCustomerIntelAfterJob(ctx context.Context, job *crmmodels.CrmIntelComputeJob, ownerOrgID primitive.ObjectID, op string, execErr error, ran bool, batch *crmIntelBatchStats) {
	if !ran && execErr == nil {
		return
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmCustomerIntelRuns)
	if !ok || coll == nil {
		logger.GetAppLogger().Warn("[CRM_INTEL_RUN] Collection crm_customer_intel_runs chưa đăng ký — bỏ qua persist")
		return
	}

	runID := primitive.NewObjectID()
	now := jobTimeMs(job)
	orderingAt := causalForOrderingMs(job)

	status := crmIntelRunStatusSuccess
	errMsg := ""
	if execErr != nil {
		status = crmIntelRunStatusFailed
		errMsg = execErr.Error()
	}

	doc := crmmodels.CrmCustomerIntelRun{
		ID:                    runID,
		OwnerOrganizationID:   ownerOrgID,
		Operation:             op,
		Status:                status,
		ParentIntelJobID:      job.ID,
		ParentDecisionEventID: job.ParentDecisionEventID,
		ComputedAt:            now,
		CausalOrderingAt:      orderingAt,
		ErrorMessage:          errMsg,
	}

	if batch != nil && batch.multi {
		doc.MultiCustomerJob = true
		doc.TotalProcessed = batch.processed
		doc.TotalFailed = batch.failed
		doc.OrgCount = batch.orgCount
		doc.ClassificationMode = batch.classificationMode
		doc.UnifiedID = ""
		doc.IntelSequence = 0
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			logger.GetAppLogger().WithError(err).Warn("[CRM_INTEL_RUN] Không ghi bản ghi job đa khách")
		}
		return
	}

	uid := payloadStrCRM(job.Payload, "unifiedId")
	doc.UnifiedID = uid

	if execErr != nil {
		doc.IntelSequence = 0
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			logger.GetAppLogger().WithError(err).WithField("unifiedId", uid).Warn("[CRM_INTEL_RUN] Không ghi bản ghi lỗi")
		}
		return
	}

	if uid == "" || ownerOrgID.IsZero() {
		doc.IntelSequence = 0
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			logger.GetAppLogger().WithError(err).Warn("[CRM_INTEL_RUN] Không ghi bản ghi thành công (không unifiedId)")
		}
		return
	}

	svc, err := NewCrmCustomerService()
	if err != nil {
		doc.MetricsSummary = bson.M{"note": "không tạo CrmCustomerService"}
		doc.IntelSequence = 0
		if _, insErr := coll.InsertOne(ctx, doc); insErr != nil {
			logger.GetAppLogger().WithError(insErr).Warn("[CRM_INTEL_RUN] Insert thất bại")
		}
		return
	}

	cust, ferr := svc.FindOne(ctx, bson.M{"unifiedId": uid, "ownerOrganizationId": ownerOrgID}, nil)
	if ferr != nil {
		doc.MetricsSummary = bson.M{"note": "không tìm thấy khách sau khi intel"}
		doc.IntelSequence = 0
		if _, insErr := coll.InsertOne(ctx, doc); insErr != nil {
			logger.GetAppLogger().WithError(insErr).Warn("[CRM_INTEL_RUN] Insert thất bại")
		}
		return
	}

	doc.CustomerMongoID = cust.ID

	var bumped crmmodels.CrmCustomer
	incErr := svc.Collection().FindOneAndUpdate(ctx,
		bson.M{"_id": cust.ID},
		bson.M{"$inc": bson.M{"intelSequence": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&bumped)
	if incErr != nil {
		logger.GetAppLogger().WithError(incErr).WithField("unifiedId", uid).Warn("[CRM_INTEL_RUN] Không $inc intelSequence — ghi run với sequence 0")
		doc.IntelSequence = 0
		doc.MetricsSummary = buildCrmCustomerMetricsSummary(&cust)
	} else {
		doc.IntelSequence = bumped.IntelSequence
		doc.MetricsSummary = buildCrmCustomerMetricsSummary(&bumped)
	}

	if _, insErr := coll.InsertOne(ctx, doc); insErr != nil {
		logger.GetAppLogger().WithError(insErr).Warn("[CRM_INTEL_RUN] Không ghi bản ghi thành công")
		return
	}

	if incErr != nil {
		return
	}

	_, uerr := svc.Collection().UpdateOne(ctx, bson.M{"_id": cust.ID}, bson.M{"$set": bson.M{
		"intelLastRunId":      runID,
		"intelLastComputedAt": now,
		"updatedAt":           now,
	}})
	if uerr != nil {
		logger.GetAppLogger().WithError(uerr).WithField("unifiedId", uid).Warn("[CRM_INTEL_RUN] Không cập nhật pointer intelLastRunId trên crm_customers")
	}
}

func jobTimeMs(_ *crmmodels.CrmIntelComputeJob) int64 {
	return time.Now().UnixMilli()
}
