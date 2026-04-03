// Package crmvc — Hàng đợi crm_intel_compute: enqueue từ consumer AI Decision; worker domain thực thi tính Intelligence.
package crmvc

import (
	"context"
	"fmt"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnqueueCrmIntelComputeFromDecisionEvent đưa job vào crm_intel_compute (không tính toán tại đây).
func EnqueueCrmIntelComputeFromDecisionEvent(ctx context.Context, parentDecisionEventID string, ownerOrgID primitive.ObjectID, payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}
	op, _ := payload["operation"].(string)
	if op == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmIntelCompute)
	if !ok {
		return fmt.Errorf("collection CrmIntelCompute chưa đăng ký")
	}
	pcopy := make(map[string]interface{}, len(payload)+1)
	for k, v := range payload {
		pcopy[k] = v
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
	ran, err := runCrmIntelComputePayload(ctx, job, op, svc, ownerOrgID)
	if err != nil {
		return err
	}
	if ran {
		uid, _ := job.Payload["unifiedId"].(string)
		_ = intelrecomputed.EmitCrmIntelRecomputed(ctx, ownerOrgID, job.ID.Hex(), job.ParentDecisionEventID, op, uid)
	}
	return nil
}

// runCrmIntelComputePayload chạy nghiệp vụ job; ran=true khi đã gọi tầng CRM thật sự (để emit crm_intel_recomputed).
func runCrmIntelComputePayload(ctx context.Context, job *crmmodels.CrmIntelComputeJob, op string, svc *CrmCustomerService, ownerOrgID primitive.ObjectID) (ran bool, err error) {
	switch op {
	case crmqueue.CrmComputeOpRefresh:
		uid, _ := job.Payload["unifiedId"].(string)
		if uid == "" || ownerOrgID.IsZero() {
			return false, nil
		}
		return true, svc.RefreshMetrics(ctx, uid, ownerOrgID)
	case crmqueue.CrmComputeOpRecalculateOne:
		uid, _ := job.Payload["unifiedId"].(string)
		if uid == "" || ownerOrgID.IsZero() {
			return false, nil
		}
		_, err := svc.RecalculateCustomerFromAllSources(ctx, uid, ownerOrgID)
		return true, err
	case crmqueue.CrmComputeOpRecalculateAll:
		if ownerOrgID.IsZero() {
			return false, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		_, err := svc.RecalculateAllCustomers(ctx, ownerOrgID, limit, poolSize)
		return true, err
	case crmqueue.CrmComputeOpRecalculateBatch:
		if ownerOrgID.IsZero() {
			return false, nil
		}
		offset := crmPayloadInt(job.Payload, "offset")
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		_, err := svc.RecalculateCustomersBatch(ctx, ownerOrgID, offset, limit, poolSize, nil, nil)
		return true, err
	case crmqueue.CrmComputeOpRecalculateMismatch:
		if ownerOrgID.IsZero() {
			return false, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 10
		}
		_, err := svc.RecalculateMismatchCustomers(ctx, ownerOrgID, limit, poolSize)
		return true, err
	case crmqueue.CrmComputeOpRecalculateOrderCountMismatch:
		if ownerOrgID.IsZero() {
			return false, nil
		}
		limit := crmPayloadInt(job.Payload, "limit")
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		_, err := svc.RecalculateOrderCountMismatchCustomers(ctx, ownerOrgID, limit, poolSize)
		return true, err
	case crmqueue.CrmComputeOpRecalculateAllOrgs:
		poolSize := crmPayloadInt(job.Payload, "poolSize")
		if poolSize <= 0 {
			poolSize = 12
		}
		_, _, _, err := svc.RecalculateAllCustomersForAllOrgs(ctx, poolSize)
		return true, err
	case crmqueue.CrmComputeOpClassificationRefresh:
		mode, _ := job.Payload["mode"].(string)
		bs := crmPayloadInt(job.Payload, "batchSize")
		if bs <= 0 {
			bs = 200
		}
		log := logger.GetAppLogger()
		n := svc.RunClassificationRefreshBatch(ctx, log, mode, bs)
		log.WithFields(map[string]interface{}{"processed": n, "mode": mode}).Debug("[CRM] Classification refresh (worker domain crm_intel_compute)")
		return true, nil
	default:
		return false, nil
	}
}
