// Package adssvc — Circuit Breaker theo FolkForm v4.1 Section 07.
// CB chạy ở cấp ACCOUNT, metrics từ meta_ad_accounts.currentMetrics (rollup từ campaigns).
// CB-1, CB-2: PAUSE toàn account. CB-3, CB-4: ALERT only, KHÔNG pause.
// Check mỗi 10p. Chỉ resume khi /resume_ads.
package adssvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const circuitBreakCollection = "ads_circuit_breaker_state"

// CircuitBreakerState lưu trạng thái circuit breaker per ad account.
type CircuitBreakerState struct {
	AdAccountId         string `bson:"adAccountId"`
	OwnerOrganizationID string `bson:"ownerOrganizationId"`
	TriggeredBy         string `bson:"triggeredBy"` // CB-1, CB-2, CB-3, CB-4
	TriggeredAt         int64  `bson:"triggeredAt"`
	Snapshot            string `bson:"snapshot,omitempty"`
}

// cbResult kết quả đánh giá CB: code, message, và có cần PAUSE hay chỉ ALERT.
type cbResult struct {
	code       string
	message    string
	shouldPause bool
}

// CheckCircuitBreaker chạy check CB-1/2/3/4 ở cấp ACCOUNT (FolkForm v4.1 S07).
// Đọc currentMetrics từ meta_ad_accounts (rollup từ campaigns).
// CB-1, CB-2: PAUSE toàn account. CB-3, CB-4: chỉ gửi ALERT, không PAUSE.
// Trả về (số account đã PAUSE, error). Chỉ resume khi /resume_ads.
func CheckCircuitBreaker(ctx context.Context) (int, error) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy meta_ad_accounts")
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy meta_campaigns")
	}

	// Lấy danh sách (adAccountId, ownerOrgId) có ít nhất 1 campaign ACTIVE
	type accKey struct {
		AdAccountId string
		OwnerOrgID  primitive.ObjectID
	}
	activeAccounts := make(map[accKey]bool)
	cursor, err := campColl.Find(ctx, bson.M{
		"$or":                 []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
		"currentMetrics.raw":  bson.M{"$exists": true},
	}, nil)
	if err != nil {
		return 0, err
	}
	for cursor.Next(ctx) {
		var camp struct {
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&camp); err != nil {
			continue
		}
		activeAccounts[accKey{camp.AdAccountId, camp.OwnerOrganizationID}] = true
	}
	cursor.Close(ctx)

	pausedCount := 0
	for k := range activeAccounts {
		if isCircuitBreakerAlreadyTriggered(ctx, k.AdAccountId, k.OwnerOrgID) {
			continue
		}
		var acc struct {
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		err := accColl.FindOne(ctx, bson.M{
			"adAccountId":         adAccountIdFilterForMeta(k.AdAccountId),
			"ownerOrganizationId": k.OwnerOrgID,
		}, nil).Decode(&acc)
		if err != nil || acc.CurrentMetrics == nil {
			continue
		}
		res := evaluateCircuitBreakerForAccount(ctx, k.AdAccountId, k.OwnerOrgID, acc.CurrentMetrics)
		if res == nil {
			continue
		}
		_, _ = SendCircuitBreakerAlert(ctx, k.AdAccountId, k.OwnerOrgID, res.code, res.message, "")
		if res.shouldPause {
			if err := pauseAllCampaigns(ctx, k.AdAccountId, k.OwnerOrgID); err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"adAccountId": k.AdAccountId,
					"code":        res.code,
				}).Error("[CIRCUIT_BREAKER] Lỗi PAUSE campaigns")
				continue
			}
			saveCircuitBreakerState(ctx, k.AdAccountId, k.OwnerOrgID, res.code, res.message)
			pausedCount++
			log.WithFields(map[string]interface{}{
				"adAccountId": k.AdAccountId,
				"code":        res.code,
				"message":     res.message,
			}).Error("🚨 [CIRCUIT_BREAKER] Đã PAUSE toàn account")
		} else {
			log.WithFields(map[string]interface{}{
				"adAccountId": k.AdAccountId,
				"code":        res.code,
				"message":     res.message,
			}).Warn("🚨 [CIRCUIT_BREAKER] ALERT only (không PAUSE)")
		}
	}
	return pausedCount, nil
}

// adAccountIdFilterForMeta trả về filter cho adAccountId — meta_ad_accounts có thể lưu "act_XXX" hoặc "XXX".
func adAccountIdFilterForMeta(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

func isCircuitBreakerAlreadyTriggered(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) bool {
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return false
	}
	var doc struct {
		CircuitBreakerTriggered string `bson:"circuitBreakerTriggered"`
	}
	err := accColl.FindOne(ctx, bson.M{
		"adAccountId":         adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}, nil).Decode(&doc)
	if err != nil {
		return false
	}
	return doc.CircuitBreakerTriggered != ""
}

// evaluateCircuitBreakerForAccount kiểm tra CB-1/2/3/4 dùng currentMetrics cấp account (từ meta_ad_accounts).
// Theo FolkForm v4.1 S07: CB-1, CB-2 → PAUSE. CB-3, CB-4 → ALERT only.
// Trả về nil nếu không trigger.
func evaluateCircuitBreakerForAccount(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}) *cbResult {
	if currentMetrics == nil {
		return nil
	}
	raw := currentMetrics
	r7d := getRaw7dFromCurrentMetrics(raw)
	meta, _ := r7d["meta"].(map[string]interface{})
	pos := getPosFromRaw7d(r7d)

	spend := toFloat64(meta, "spend")
	mess := toInt64(meta, "mess")
	r2h := getRaw2hFromCurrentMetrics(raw)
	r30p := getRaw30pFromCurrentMetrics(raw)

	cfg, _ := adsconfig.GetConfigForCampaign(ctx, adAccountId, ownerOrgID)
	common := adsconfig.GetCommon(cfg)
	threshold := common.Cb4MessThreshold
	if threshold <= 0 {
		threshold = 50
	}

	// CB-4: Pancake_orders_2h = 0 VÀ FB_Mess_2h > threshold — ALERT only, KHÔNG pause (FolkForm S07).
	if r2h != nil {
		o2h := toInt64(r2h, "orders")
		m2h := toInt64(r2h, "mess")
		if o2h == 0 && m2h > int64(threshold) {
			return &cbResult{"CB-4", fmt.Sprintf("Pancake 0 đơn, FB Mess 2h=%d — nghi Pancake lỗi. Yêu cầu Sales kiểm tra", m2h), false}
		}
	}

	// CB-3: Zero delivery 30p — Spend=0 trong 30p dù Active. Ưu tiên snapshot; fallback mess/orders proxy. ALERT only.
	spend30pSnap, _, okSpend30 := metasvc.GetSpendImpressions30pCurrentSlot(ctx, adAccountId, ownerOrgID)
	if okSpend30 && spend30pSnap == 0 && spend > 0 {
		loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
		h := time.Now().In(loc).Hour()
		if h >= 8 && h <= 22 {
			return &cbResult{"CB-3", "Zero delivery 30p — Spend=0 (từ snapshot) trong giờ hoạt động. FB kỹ thuật lỗi hoặc camp disapprove?", false}
		}
	}
	if !okSpend30 && r30p != nil {
		m30p := toInt64(r30p, "mess")
		o30p := toInt64(r30p, "orders")
		if m30p == 0 && o30p == 0 && mess > 30 {
			loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
			h := time.Now().In(loc).Hour()
			if h >= 8 && h <= 22 {
				return &cbResult{"CB-3", "Zero delivery 30p — mess=0, orders=0 trong giờ hoạt động. FB kỹ thuật lỗi hoặc camp disapprove?", false}
			}
		}
	}

	// CB-1: Spend spike + ROAS thấp. FolkForm: spend_30p > spend_yesterday_cùng_30p × 4 VÀ ROAS_1h < 1.5.
	// Ưu tiên snapshot (chính xác); fallback proxy ROAS_2h từ spend_7d.
	if r2h != nil {
		rev2h := toFloat64(r2h, "revenue")
		r1h := getRaw1hFromCurrentMetrics(raw)
		rev1h := 0.0
		if r1h != nil {
			rev1h = toFloat64(r1h, "revenue")
		}
		// Thử dùng snapshot: spend_30p, spend_yesterday, spend_1h
		spend30p, spendYest, ok30 := metasvc.GetSpend30pAndYesterday(ctx, adAccountId, ownerOrgID)
		spend1h, ok1h := metasvc.GetSpend1hFromSnapshots(ctx, adAccountId, ownerOrgID)
		if ok30 && spendYest > 0 && spend30p > spendYest*4 {
			roas1h := 0.0
			if ok1h && spend1h > 0 && rev1h > 0 {
				roas1h = rev1h / spend1h
			} else if spend > 500_000 && rev2h > 0 {
				spend2hEst := spend / 84
				if spend2hEst > 50_000 {
					roas1h = rev2h / spend2hEst
				}
			}
			if roas1h > 0 && roas1h < 1.5 {
				return &cbResult{"CB-1", fmt.Sprintf("Spend 30p %.0f > yesterday×4 (%.0f), ROAS 1h %.2f < 1.5 — đốt tiền gấp 4x không ra đơn. PAUSE", spend30p, spendYest, roas1h), true}
			}
		}
		// Fallback: proxy ROAS_2h (không có snapshot đủ)
		if spend > 500_000 {
			spend2hEst := spend / 84
			if spend2hEst > 50_000 && rev2h > 0 {
				roas2h := rev2h / spend2hEst
				if roas2h < 1.5 {
					return &cbResult{"CB-1", fmt.Sprintf("ROAS 2h proxy %.2f < 1.5 — spend spike, hiệu quả thấp. PAUSE toàn account", roas2h), true}
				}
			}
		}
	}

	// CB-2: CPM_account_avg_30p > CPM_3day_avg × 3 VÀ Impressions giảm. Dùng snapshot khi có.
	spend30p, imp30p, ok30 := metasvc.GetSpendImpressions30pCurrentSlot(ctx, adAccountId, ownerOrgID)
	cpm3day, ok3day := metasvc.GetCPM3dayAvgFromInsights(ctx, adAccountId, ownerOrgID)
	if ok30 && ok3day && cpm3day > 0 && imp30p > 0 {
		cpm30p := spend30p / (imp30p / 1000)
		if cpm30p > cpm3day*3 {
			return &cbResult{"CB-2", fmt.Sprintf("CPM 30p %.0fk > CPM 3day×3 (%.0fk) — auction bất thường. PAUSE", cpm30p/1000, cpm3day/1000), true}
		}
	}
	_ = pos

	return nil
}

func getRaw7dFromCurrentMetrics(current map[string]interface{}) map[string]interface{} {
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		return map[string]interface{}{}
	}
	r7d, _ := raw["7d"].(map[string]interface{})
	if r7d == nil {
		return map[string]interface{}{}
	}
	return r7d
}

func getRaw2hFromCurrentMetrics(current map[string]interface{}) map[string]interface{} {
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		return nil
	}
	r2h, _ := raw["2h"].(map[string]interface{})
	return r2h
}

func getRaw1hFromCurrentMetrics(current map[string]interface{}) map[string]interface{} {
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		return nil
	}
	r1h, _ := raw["1h"].(map[string]interface{})
	return r1h
}

// getRaw30pFromCurrentMetrics trả về raw.30p từ currentMetrics (orders, revenue, mess).
func getRaw30pFromCurrentMetrics(current map[string]interface{}) map[string]interface{} {
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		return nil
	}
	r30p, _ := raw["30p"].(map[string]interface{})
	return r30p
}

func getPosFromRaw7d(r7d map[string]interface{}) map[string]interface{} {
	pancake, _ := r7d["pancake"].(map[string]interface{})
	if pancake == nil {
		return nil
	}
	pos, _ := pancake["pos"].(map[string]interface{})
	return pos
}

func toFloat64(m map[string]interface{}, k string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func toInt64(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func pauseAllCampaigns(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) error {
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy meta_campaigns")
	}
	cursor, err := campColl.Find(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
		"$or":                 []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
	}, nil)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc struct {
			CampaignId string `bson:"campaignId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		_, err := Propose(ctx, &ProposeInput{
			ActionType:   "PAUSE",
			AdAccountId:  adAccountId,
			CampaignId:   doc.CampaignId,
			Reason:       "Circuit Breaker — PAUSE toàn account",
			RuleCode:     "circuit_breaker",
		}, ownerOrgID, getProposeBaseURL())
		if err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"campaignId": doc.CampaignId,
			}).Warn("[CIRCUIT_BREAKER] Lỗi tạo proposal PAUSE")
		}
	}
	return nil
}

func saveCircuitBreakerState(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, code, snapshot string) {
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	accColl.UpdateOne(ctx,
		bson.M{"adAccountId": adAccountIdFilterForMeta(adAccountId), "ownerOrganizationId": ownerOrgID},
		bson.M{"$set": bson.M{
			"circuitBreakerTriggered": code,
			"circuitBreakerAt":        time.Now().UnixMilli(),
			"circuitBreakerSnapshot":  snapshot,
		}},
	)
}
