// Package crmvc — Hàng đợi crm_intel_compute: enqueue từ consumer AI Decision; worker domain thực thi tính Intelligence.
package crmvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnqueueCrmIntelComputeFromDecisionEvent đưa job vào crm_intel_compute (không tính toán tại đây).
// traceID / correlationID ghi vào payload để worker domain Publish timeline cùng luồng AID.
// bus — bản sao eventType/eventSource/pipelineStage từ decision_events_queue (có thể nil).
func EnqueueCrmIntelComputeFromDecisionEvent(ctx context.Context, parentDecisionEventID string, ownerOrgID primitive.ObjectID, payload map[string]interface{}, traceID, correlationID string, bus *crmqueue.DomainQueueBusFields) error {
	if payload == nil {
		return nil
	}
	op, _ := payload["operation"].(string)
	if op == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerIntelCompute)
	if !ok {
		return fmt.Errorf("collection CustomerIntelCompute chưa đăng ký")
	}
	pcopy := make(map[string]interface{}, len(payload)+3)
	for k, v := range payload {
		pcopy[k] = v
	}
	if tid := strings.TrimSpace(traceID); tid != "" {
		pcopy["traceId"] = tid
	}
	if cid := strings.TrimSpace(correlationID); cid != "" {
		pcopy["correlationId"] = cid
	}
	orgID := ownerOrgID
	if orgID.IsZero() {
		if hex, ok := payload["ownerOrgIdHex"].(string); ok && hex != "" {
			oid, err := primitive.ObjectIDFromHex(hex)
			if err == nil {
				orgID = oid
			}
		}
	}
	now := time.Now().UnixMilli()
	job := &crmmodels.CrmIntelComputeJob{
		ID:                    primitive.NewObjectID(),
		Payload:               pcopy,
		OwnerOrganizationID:   orgID,
		ParentDecisionEventID: parentDecisionEventID,
		CreatedAt:             now,
	}
	if bus != nil {
		job.EventType = strings.TrimSpace(bus.EventType)
		job.EventSource = strings.TrimSpace(bus.EventSource)
		job.PipelineStage = strings.TrimSpace(bus.PipelineStage)
		job.OwnerDomain = strings.TrimSpace(bus.OwnerDomain)
		job.ProcessorDomain = strings.TrimSpace(bus.ProcessorDomain)
		job.EnqueueSourceDomain = strings.TrimSpace(bus.EnqueueSourceDomain)
		job.E2EStage = strings.TrimSpace(bus.E2EStage)
		job.E2EStepID = strings.TrimSpace(bus.E2EStepID)
	}
	_, err := coll.InsertOne(ctx, job)
	return err
}

func crmPayloadInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

// RunCrmIntelComputeJob thực thi một job (gọi từ worker domain CRM).
func RunCrmIntelComputeJob(ctx context.Context, job *crmmodels.CrmIntelComputeJob) error {
	if job == nil || job.Payload == nil {
		return nil
	}
	op, _ := job.Payload["operation"].(string)
	if op == "" {
		return nil
	}
	svc, err := NewCrmCustomerService()
	if err != nil {
		return err
	}
	ownerOrgID := job.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := job.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			oid, err := primitive.ObjectIDFromHex(hex)
			if err == nil {
				ownerOrgID = oid
			}
		}
	}
	ran, batchStats, execErr := runCrmIntelComputePayload(ctx, job, op, svc, ownerOrgID)
	persistCrmCustomerIntelAfterJob(ctx, job, ownerOrgID, op, execErr, ran, batchStats)
	if execErr != nil {
		return execErr
	}
	if ran {
		uid, _ := job.Payload["unifiedId"].(string)
		_ = intelrecomputed.EmitCrmIntelRecomputed(ctx, ownerOrgID, job.ID.Hex(), job.ParentDecisionEventID, op, uid)
	}
	return nil
}

// runCrmIntelComputePayload chạy nghiệp vụ job; ran=true khi đã gọi tầng CRM thật sự (để emit crm_intel_recomputed).
// batchStats != nil và multi=true: ghi một bản ghi lịch sử cho cả job (không cập nhật intelLastRunId từng khách).
func runCrmIntelComputePayload(ctx context.Context, job *crmmodels.CrmIntelComputeJob, op string, svc *CrmCustomerService, ownerOrgID primitive.ObjectID) (ran bool, batchStats *crmIntelBatchStats, err error) {
	switch op {
	case crmqueue.CrmComputeOpRefresh:
		uid, _ := job.Payload["unifiedId"].(string)
		if uid == "" || ownerOrgID.IsZero() {
			return false, nil, nil
		}
		return true, nil, svc.RefreshMetrics(ctx, uid, ownerOrgID)
	case crmqueue.CrmComputeOpRecalculateOne:
		uid, _ := job.Payload["unifiedId"].(string)
		if uid == "" || ownerOrgID.IsZero() {
			return false, nil, nil
		}
		_, err := svc.RecalculateCustomerFromAllSources(ctx, uid, ownerOrgID)
		return true, nil, err
	case crmqueue.CrmComputeOpRecalculateAll:
		if ownerOrgID.IsZero() {
			return false, nil, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		res, err := svc.RecalculateAllCustomers(ctx, ownerOrgID, limit, poolSize)
		if err != nil {
			return true, &crmIntelBatchStats{multi: true}, err
		}
		return true, &crmIntelBatchStats{multi: true, processed: res.TotalProcessed, failed: res.TotalFailed}, nil
	case crmqueue.CrmComputeOpRecalculateBatch:
		if ownerOrgID.IsZero() {
			return false, nil, nil
		}
		offset := crmPayloadInt(job.Payload, "offset")
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		res, err := svc.RecalculateCustomersBatch(ctx, ownerOrgID, offset, limit, poolSize, nil, nil)
		if err != nil {
			return true, &crmIntelBatchStats{multi: true}, err
		}
		return true, &crmIntelBatchStats{multi: true, processed: res.TotalProcessed, failed: res.TotalFailed}, nil
	case crmqueue.CrmComputeOpRecalculateMismatch:
		if ownerOrgID.IsZero() {
			return false, nil, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 10
		}
		res, err := svc.RecalculateMismatchCustomers(ctx, ownerOrgID, limit, poolSize)
		if err != nil {
			return true, &crmIntelBatchStats{multi: true}, err
		}
		return true, &crmIntelBatchStats{multi: true, processed: res.TotalProcessed, failed: res.TotalFailed}, nil
	case crmqueue.CrmComputeOpRecalculateOrderCountMismatch:
		if ownerOrgID.IsZero() {
			return false, nil, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		res, err := svc.RecalculateOrderCountMismatchCustomers(ctx, ownerOrgID, limit, poolSize)
		if err != nil {
			return true, &crmIntelBatchStats{multi: true}, err
		}
		return true, &crmIntelBatchStats{multi: true, processed: res.TotalProcessed, failed: res.TotalFailed}, nil
	case crmqueue.CrmComputeOpRecalculateAllOrgs:
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		tp, tf, oc, err := svc.RecalculateAllCustomersForAllOrgs(ctx, poolSize)
		if err != nil {
			return true, &crmIntelBatchStats{multi: true}, err
		}
		return true, &crmIntelBatchStats{multi: true, processed: tp, failed: tf, orgCount: oc}, nil
	case crmqueue.CrmComputeOpClassificationRefresh:
		mode, _ := job.Payload["mode"].(string)
		bs := crmPayloadInt(job.Payload, "batchSize")
		if bs <= 0 {
			bs = 200
		}
		log := logger.GetAppLogger()
		n := svc.RunClassificationRefreshBatch(ctx, log, mode, bs)
		log.WithFields(map[string]interface{}{"processed": n, "mode": mode}).Debug("[CRM] Classification refresh (worker domain crm_intel_compute)")
		return true, &crmIntelBatchStats{multi: true, processed: n, classificationMode: mode}, nil
	default:
		return false, nil, nil
	}
}
