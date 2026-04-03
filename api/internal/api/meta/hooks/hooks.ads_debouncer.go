// Package metahooks — Debounce/Throttle Ads Intelligence dùng DB để theo dõi.
package metahooks

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/adsintel"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// normalizeAdAccountForIntelKey chuẩn hóa act_XXX vs số thuần để cùng tài khoản không tách hai slot.
func normalizeAdAccountForIntelKey(adAccountId string) string {
	adAccountId = strings.TrimSpace(adAccountId)
	if strings.HasPrefix(strings.ToLower(adAccountId), "act_") {
		return strings.TrimPrefix(adAccountId, "act_")
	}
	return adAccountId
}

// CampaignIntelThrottleKey — khóa giảm chấn theo entity tính lại.
func CampaignIntelThrottleKey(ownerOrgID primitive.ObjectID, adAccountId, recalcObjectType, recalcObjectID string) string {
	acc := normalizeAdAccountForIntelKey(adAccountId)
	return "c_intel|" + ownerOrgID.Hex() + "|" + acc + "|" + strings.TrimSpace(recalcObjectType) + "|" + strings.TrimSpace(recalcObjectID)
}

type campaignIntelDebounceDoc struct {
	DebounceKey       string             `bson:"debounceKey"`
	OwnerOrgID        primitive.ObjectID `bson:"ownerOrgId"`
	AdAccountID       string             `bson:"adAccountId"`
	RecalcObjectType  string             `bson:"recalcObjectType"`
	RecalcObjectID    string             `bson:"recalcObjectId"`
	Status            string             `bson:"status"` // scheduled|emitted|emit_failed
	UrgentRequested   bool               `bson:"urgentRequested"`
	MinIntervalMs     int64              `bson:"minIntervalMs"`
	TrailingMs        int64              `bson:"trailingMs"`
	FirstSeenAt       int64              `bson:"firstSeenAt"`
	LastSeenAt        int64              `bson:"lastSeenAt"`
	NextEmitAt        int64              `bson:"nextEmitAt"`
	LastEmitAt        int64              `bson:"lastEmitAt"`
	LastEmitEventID   string             `bson:"lastEmitEventId,omitempty"`
	LastEmitStatus    string             `bson:"lastEmitStatus,omitempty"`
	LastEmitError     string             `bson:"lastEmitError,omitempty"`
	EmitCount         int                `bson:"emitCount"`
	SourceKinds       []string           `bson:"sourceKinds,omitempty"`
	UpdatedAt         int64              `bson:"updatedAt"`
	CreatedAt         int64              `bson:"createdAt"`
}

var (
	campaignTimerMu sync.Mutex
	campaignTimers  = make(map[string]*time.Timer)
)

type campaignIntelDebounceRequest struct {
	OwnerOrgID       primitive.ObjectID
	AdAccountID      string
	RecalcObjectType string
	RecalcObjectID   string
	SourceKind       string
	Urgent           bool
}

func minIntervalMsForObjectType(objectType string) int64 {
	ot := strings.TrimSpace(strings.ToLower(objectType))
	switch ot {
	case "campaign":
		return int64(envInt("ADS_INTEL_DEBOUNCE_CAMPAIGN_MS", adsintel.DebounceMsInsightBatch))
	case "ad_account":
		return int64(envInt("ADS_INTEL_DEBOUNCE_AD_ACCOUNT_MS", adsintel.DebounceMsInsightBatch))
	case "adset":
		return int64(envInt("ADS_INTEL_DEBOUNCE_ADSET_MS", adsintel.DebounceMsInsightBatch))
	case "ad":
		return int64(envInt("ADS_INTEL_DEBOUNCE_AD_MS", adsintel.DebounceMsInsightBatch))
	default:
		return int64(envInt("ADS_INTEL_DEBOUNCE_DEFAULT_MS", adsintel.DebounceMsInsightBatch))
	}
}

func trailingMsForObjectType(objectType string) int64 {
	ot := strings.TrimSpace(strings.ToLower(objectType))
	// Mặc định trailing = 15 phút (cùng cửa sổ min batch insight); env ghi đè từng loại.
	switch ot {
	case "campaign":
		return int64(envInt("ADS_INTEL_DEBOUNCE_TRAILING_CAMPAIGN_MS", adsintel.DebounceMsInsightBatch))
	case "ad_account":
		return int64(envInt("ADS_INTEL_DEBOUNCE_TRAILING_AD_ACCOUNT_MS", adsintel.DebounceMsInsightBatch))
	case "adset":
		return int64(envInt("ADS_INTEL_DEBOUNCE_TRAILING_ADSET_MS", adsintel.DebounceMsInsightBatch))
	case "ad":
		return int64(envInt("ADS_INTEL_DEBOUNCE_TRAILING_AD_MS", adsintel.DebounceMsInsightBatch))
	default:
		return int64(envInt("ADS_INTEL_DEBOUNCE_TRAILING_DEFAULT_MS", adsintel.DebounceMsInsightBatch))
	}
}

func envInt(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// enqueueCampaignIntelRecomputeDebounced ghi trạng thái debounce vào DB rồi canh giờ emit vào decision_events_queue.
func enqueueCampaignIntelRecomputeDebounced(ctx context.Context, req *campaignIntelDebounceRequest) {
	if req == nil || req.OwnerOrgID.IsZero() || strings.TrimSpace(req.RecalcObjectID) == "" || strings.TrimSpace(req.AdAccountID) == "" {
		return
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RecomputeDebounceQueue)
	if !ok || coll == nil {
		return
	}
	key := CampaignIntelThrottleKey(req.OwnerOrgID, req.AdAccountID, req.RecalcObjectType, req.RecalcObjectID)
	now := time.Now().UnixMilli()
	minI := minIntervalMsForObjectType(req.RecalcObjectType)
	trailing := trailingMsForObjectType(req.RecalcObjectType)

	var prev campaignIntelDebounceDoc
	_ = coll.FindOne(ctx, bson.M{"debounceKey": key}).Decode(&prev)

	nextEmitAt := now
	if req.Urgent {
		nextEmitAt = now
	} else if prev.LastEmitAt <= 0 {
		nextEmitAt = now + minI
	} else {
		cooldownUntil := prev.LastEmitAt + minI
		if now < cooldownUntil {
			nextEmitAt = cooldownUntil
		} else {
			nextEmitAt = now + trailing
		}
	}

	update := bson.M{
		"$set": bson.M{
			"ownerOrgId":       req.OwnerOrgID,
			"adAccountId":      normalizeAdAccountForIntelKey(req.AdAccountID),
			"recalcObjectType": strings.TrimSpace(req.RecalcObjectType),
			"recalcObjectId":   strings.TrimSpace(req.RecalcObjectID),
			"status":           "scheduled",
			"urgentRequested":  req.Urgent || prev.UrgentRequested,
			"minIntervalMs":    minI,
			"trailingMs":       trailing,
			"lastSeenAt":       now,
			"nextEmitAt":       nextEmitAt,
			"updatedAt":        now,
		},
		"$setOnInsert": bson.M{
			"debounceKey": key,
			"firstSeenAt": now,
			"createdAt":   now,
			"emitCount":   0,
		},
	}
	if sk := strings.TrimSpace(req.SourceKind); sk != "" {
		update["$addToSet"] = bson.M{"sourceKinds": sk}
	}
	_, err := coll.UpdateOne(ctx, bson.M{"debounceKey": key}, update, options.Update().SetUpsert(true))
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("metahooks: không ghi được hàng đợi debounce Ads Intelligence")
		return
	}

	scheduleDbEmitTimer(key, time.Duration(maxInt64(0, nextEmitAt-now))*time.Millisecond)
}

func scheduleDbEmitTimer(debounceKey string, d time.Duration) {
	campaignTimerMu.Lock()
	if t, ok := campaignTimers[debounceKey]; ok && t != nil {
		t.Stop()
	}
	campaignTimers[debounceKey] = time.AfterFunc(d, func() {
		processDueCampaignIntelDebounce(debounceKey)
	})
	campaignTimerMu.Unlock()
}

func processDueCampaignIntelDebounce(debounceKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RecomputeDebounceQueue)
	if !ok || coll == nil {
		return
	}
	var doc campaignIntelDebounceDoc
	if err := coll.FindOne(ctx, bson.M{"debounceKey": debounceKey}).Decode(&doc); err != nil {
		return
	}
	now := time.Now().UnixMilli()
	if doc.NextEmitAt > now {
		scheduleDbEmitTimer(debounceKey, time.Duration(doc.NextEmitAt-now)*time.Millisecond)
		return
	}
	if strings.TrimSpace(doc.RecalcObjectType) == "" || strings.TrimSpace(doc.RecalcObjectID) == "" || doc.OwnerOrgID.IsZero() {
		return
	}
	eventID, err := aidecisionsvc.EmitAdsIntelligenceRecomputeRequested(
		ctx,
		doc.RecalcObjectType,
		doc.RecalcObjectID,
		doc.AdAccountID,
		doc.OwnerOrgID,
		"campaign_intel_throttle",
		metasvc.RecomputeModeFull,
	)
	status := "emitted"
	errMsg := ""
	lastEmitStatus := "ok"
	if err != nil {
		status = "emit_failed"
		lastEmitStatus = "error"
		errMsg = truncateForDebounceDoc(err.Error(), 600)
		logger.GetAppLogger().WithError(err).WithField("debounceKey", debounceKey).Warn("metahooks: emit recompute từ hàng đợi debounce thất bại")
	}
	update := bson.M{
		"$set": bson.M{
			"status":         status,
			"lastEmitAt":     now,
			"lastEmitEventId": eventID,
			"lastEmitStatus": lastEmitStatus,
			"lastEmitError":  errMsg,
			"urgentRequested": false,
			"updatedAt":      now,
		},
		"$inc": bson.M{"emitCount": 1},
	}
	_, _ = coll.UpdateOne(ctx, bson.M{"debounceKey": debounceKey}, update)
}

func truncateForDebounceDoc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
