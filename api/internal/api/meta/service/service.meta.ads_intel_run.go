// Package metasvc — Lớp A ads_meta_intel_runs + pointer intel trên meta_campaigns (chuẩn hai lớp A/B, đồng bộ order_intel_runs).
package metasvc

import (
	"context"
	"strings"
	"time"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func causalMsForAdsIntelJob(job *adsmodels.AdsIntelComputeJob) int64 {
	if job == nil {
		return time.Now().UnixMilli()
	}
	if job.CausalOrderingAtMs > 0 {
		return job.CausalOrderingAtMs
	}
	return time.Now().UnixMilli()
}

func normalizeAdsIntelCausalMs(ms int64) int64 {
	if ms > 0 {
		return ms
	}
	return time.Now().UnixMilli()
}

func buildAdsMetaIntelSummary(cm map[string]interface{}) bson.M {
	if cm == nil {
		return nil
	}
	out := bson.M{}
	for _, k := range []string{"raw", "layer1", "layer2", "layer3", "alertFlags"} {
		if v, ok := cm[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// loadCampaignIntelSummary đọc meta_campaigns theo campaignId (+ adAccountId nếu có) và trả về tóm tắt + _id Mongo.
func loadCampaignIntelSummary(ctx context.Context, campaignID, adAccountID string, ownerOrgID primitive.ObjectID) (summary bson.M, campMongoID primitive.ObjectID) {
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" || ownerOrgID.IsZero() {
		return nil, primitive.NilObjectID
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok || coll == nil {
		return nil, primitive.NilObjectID
	}
	filter := bson.M{
		"campaignId":          campaignID,
		"ownerOrganizationId": ownerOrgID,
	}
	if adAcc := strings.TrimSpace(adAccountID); adAcc != "" {
		filter["adAccountId"] = adAcc
	}
	var doc struct {
		ID             primitive.ObjectID     `bson:"_id"`
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return nil, primitive.NilObjectID
	}
	return buildAdsMetaIntelSummary(doc.CurrentMetrics), doc.ID
}

// insertAdsMetaIntelRunAndMaybeBumpCampaign ghi run; idempotent theo parentIntelJobId+ownerOrganizationId; success + campMongoID thì $inc sequence + pointer.
func insertAdsMetaIntelRunAndMaybeBumpCampaign(ctx context.Context, run *adsmodels.AdsMetaIntelRun, job *adsmodels.AdsIntelComputeJob, campMongoID primitive.ObjectID, bumpPointer bool) {
	if run == nil || job == nil || job.OwnerOrganizationID.IsZero() {
		return
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaIntelRuns)
	if !ok || coll == nil {
		logger.GetAppLogger().Warn("📋 [ADS_META_INTEL_RUN] Collection ads_meta_intel_runs chưa đăng ký — bỏ qua persist")
		return
	}

	runID := run.ID
	_, insErr := coll.InsertOne(ctx, run)
	if insErr != nil && mongo.IsDuplicateKeyError(insErr) {
		var existing adsmodels.AdsMetaIntelRun
		q := bson.M{"parentIntelJobId": job.ID, "ownerOrganizationId": job.OwnerOrganizationID}
		if err := coll.FindOne(ctx, q).Decode(&existing); err != nil {
			logger.GetAppLogger().WithError(err).Warn("📋 [ADS_META_INTEL_RUN] Trùng khóa job nhưng không đọc lại được run")
			return
		}
		runID = existing.ID
		// Retry: lần trước failed/skipped, lần này success — cập nhật bản ghi để audit đúng.
		if (existing.Status == adsmodels.AdsMetaIntelRunStatusFailed || existing.Status == adsmodels.AdsMetaIntelRunStatusSkipped) && run.Status == adsmodels.AdsMetaIntelRunStatusSuccess {
			_, _ = coll.UpdateOne(ctx, bson.M{"_id": existing.ID}, bson.M{"$set": bson.M{
				"status":           run.Status,
				"errorMessage":     run.ErrorMessage,
				"intelSummary":     run.IntelSummary,
				"computedAt":       run.ComputedAt,
				"causalOrderingAt": run.CausalOrderingAt,
				"campaignId":       run.CampaignId,
				"adAccountId":      run.AdAccountId,
				"objectType":       run.ObjectType,
				"objectId":         run.ObjectID,
				"jobKind":          run.JobKind,
				"operation":        run.Operation,
			}})
		} else {
			bumpPointer = false
		}
	} else if insErr != nil {
		logger.GetAppLogger().WithError(insErr).Warn("📋 [ADS_META_INTEL_RUN] Insert run thất bại")
		return
	}

	if !bumpPointer || run.Status != adsmodels.AdsMetaIntelRunStatusSuccess || campMongoID.IsZero() || job.OwnerOrganizationID.IsZero() {
		return
	}

	campColl, okC := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !okC || campColl == nil {
		return
	}

	var seq int64
	var bumped struct {
		IntelSequence int64 `bson:"intelSequence"`
	}
	incErr := campColl.FindOneAndUpdate(ctx,
		bson.M{"_id": campMongoID, "ownerOrganizationId": job.OwnerOrganizationID},
		bson.M{"$inc": bson.M{"intelSequence": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&bumped)
	if incErr != nil {
		logger.GetAppLogger().WithError(incErr).WithField("campaignMongoId", campMongoID.Hex()).Warn("📋 [ADS_META_INTEL_RUN] Không $inc intelSequence trên meta_campaigns")
	} else {
		seq = bumped.IntelSequence
		_, _ = coll.UpdateOne(ctx, bson.M{"_id": runID}, bson.M{"$set": bson.M{"intelSequence": seq}})
	}

	_, uerr := campColl.UpdateOne(ctx,
		bson.M{"_id": campMongoID, "ownerOrganizationId": job.OwnerOrganizationID},
		bson.M{"$set": bson.M{
			"intelLastRunId":      runID,
			"intelLastComputedAt": run.ComputedAt,
		}},
	)
	if uerr != nil {
		logger.GetAppLogger().WithError(uerr).WithField("campaignMongoId", campMongoID.Hex()).Warn("📋 [ADS_META_INTEL_RUN] Không cập nhật pointer intelLastRunId trên meta_campaigns")
	}
}

func persistAdsMetaIntelAfterRecomputeOne(ctx context.Context, job *adsmodels.AdsIntelComputeJob, execErr error, nowMs int64) {
	if job == nil || job.OwnerOrganizationID.IsZero() {
		return
	}
	campaignID, adAccountID, ok := resolveCampaignIDAfterRecomputeJob(ctx, job)
	orderingAt := causalMsForAdsIntelJob(job)

	status := adsmodels.AdsMetaIntelRunStatusSuccess
	errMsg := ""
	if execErr != nil {
		status = adsmodels.AdsMetaIntelRunStatusFailed
		errMsg = execErr.Error()
	} else if !ok {
		status = adsmodels.AdsMetaIntelRunStatusSkipped
	}

	var summary bson.M
	var campMongoID primitive.ObjectID
	if execErr == nil && ok {
		summary, campMongoID = loadCampaignIntelSummary(ctx, campaignID, adAccountID, job.OwnerOrganizationID)
	}

	op := strings.TrimSpace(job.Source)
	if op == "" {
		op = "ads_job_intel"
	}

	run := &adsmodels.AdsMetaIntelRun{
		ID:                    primitive.NewObjectID(),
		OwnerOrganizationID:   job.OwnerOrganizationID,
		CampaignId:            campaignID,
		AdAccountId:           adAccountID,
		JobKind:               adsmodels.AdsIntelComputeKindRecomputeOne,
		ObjectType:            job.ObjectType,
		ObjectID:              job.ObjectID,
		Operation:             op,
		Status:                status,
		ParentIntelJobID:      job.ID,
		ParentDecisionEventID: job.ParentDecisionEventID,
		ComputedAt:            nowMs,
		CausalOrderingAt:      orderingAt,
		ErrorMessage:          errMsg,
		IntelSummary:          summary,
	}
	bump := execErr == nil && ok && !campMongoID.IsZero()
	insertAdsMetaIntelRunAndMaybeBumpCampaign(ctx, run, job, campMongoID, bump)
}

func persistAdsMetaIntelAfterRecalculateAll(ctx context.Context, job *adsmodels.AdsIntelComputeJob, execErr error, nowMs int64) {
	if job == nil || job.OwnerOrganizationID.IsZero() {
		return
	}
	orderingAt := causalMsForAdsIntelJob(job)
	status := adsmodels.AdsMetaIntelRunStatusSuccess
	errMsg := ""
	if execErr != nil {
		status = adsmodels.AdsMetaIntelRunStatusFailed
		errMsg = execErr.Error()
	}
	run := &adsmodels.AdsMetaIntelRun{
		ID:                    primitive.NewObjectID(),
		OwnerOrganizationID:   job.OwnerOrganizationID,
		JobKind:               adsmodels.AdsIntelComputeKindRecalculateAll,
		Operation:             "recalculate_all",
		Status:                status,
		ParentIntelJobID:      job.ID,
		ParentDecisionEventID: job.ParentDecisionEventID,
		ComputedAt:            nowMs,
		CausalOrderingAt:      orderingAt,
		ErrorMessage:          errMsg,
		MultiCampaignJob:      true,
	}
	insertAdsMetaIntelRunAndMaybeBumpCampaign(ctx, run, job, primitive.NilObjectID, false)
}

func persistAdsMetaIntelAfterContextReady(ctx context.Context, job *adsmodels.AdsIntelComputeJob, execErr error, nowMs int64) {
	if job == nil || job.OwnerOrganizationID.IsZero() {
		return
	}
	orderingAt := causalMsForAdsIntelJob(job)
	campaignID := strings.TrimSpace(job.ObjectID)
	adAccountID := strings.TrimSpace(job.AdAccountID)

	status := adsmodels.AdsMetaIntelRunStatusSuccess
	errMsg := ""
	if execErr != nil {
		status = adsmodels.AdsMetaIntelRunStatusFailed
		errMsg = execErr.Error()
	}

	var summary bson.M
	var campMongoID primitive.ObjectID
	if execErr == nil && campaignID != "" {
		summary, campMongoID = loadCampaignIntelSummary(ctx, campaignID, adAccountID, job.OwnerOrganizationID)
	}

	run := &adsmodels.AdsMetaIntelRun{
		ID:                    primitive.NewObjectID(),
		OwnerOrganizationID:   job.OwnerOrganizationID,
		CampaignId:            campaignID,
		AdAccountId:           adAccountID,
		JobKind:               adsmodels.AdsIntelComputeKindContextReady,
		Operation:             "context_ready",
		Status:                status,
		ParentIntelJobID:      job.ID,
		ParentDecisionEventID: job.ParentDecisionEventID,
		ComputedAt:            nowMs,
		CausalOrderingAt:      orderingAt,
		ErrorMessage:          errMsg,
		IntelSummary:          summary,
	}
	// context_ready không cập nhật pointer intel trên campaign (chỉ đọc snapshot + emit event).
	insertAdsMetaIntelRunAndMaybeBumpCampaign(ctx, run, job, campMongoID, false)
}
