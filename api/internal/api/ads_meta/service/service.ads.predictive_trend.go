// Package adssvc — Predictive Trend Alerts (FolkForm v4.1 Section 2.4).
// Linear regression 7 ngày, chạy 07:45, R² ≥ 0.6 mới gửi alert.
package adssvc

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	adsadaptive "meta_commerce/internal/api/ads_meta/adaptive"
	adsconfig "meta_commerce/internal/api/ads_meta/config"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	// R2MinConfidence ngưỡng R² tối thiểu để gửi alert (data đủ consistent).
	R2MinConfidence = 0.6
	// PredictiveTrendDays số ngày dùng cho regression.
	PredictiveTrendDays = 7
)

// linearRegression tính hồi quy tuyến tính y = mx + b. x = 0..n-1.
// Trả về m, b, r2. R² = 1 - SS_res/SS_tot.
func linearRegression(x, y []float64) (m, b, r2 float64) {
	n := len(x)
	if n != len(y) || n < 2 {
		return 0, 0, 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}
	denom := float64(n)*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		return 0, 0, 0
	}
	m = (float64(n)*sumXY - sumX*sumY) / denom
	b = (sumY - m*sumX) / float64(n)

	meanY := sumY / float64(n)
	var ssTot, ssRes float64
	for i := 0; i < n; i++ {
		yi := m*x[i] + b
		ssTot += (y[i] - meanY) * (y[i] - meanY)
		ssRes += (y[i] - yi) * (y[i] - yi)
	}
	if ssTot < 1e-10 {
		return m, b, 0
	}
	r2 = 1 - ssRes/ssTot
	return m, b, r2
}

// daySeriesRow một dòng trong chuỗi 7 ngày (từ meta_ad_insights).
type daySeriesRow struct {
	Date      string  `bson:"date"`
	Frequency float64 `bson:"frequency"`
	CPM       float64 `bson:"cpm"`
	CTR       float64 `bson:"ctr"`
	Spend     float64 `bson:"spend"`
	Mess      int64   `bson:"mess"`
}

// getCampaign7DaySeries lấy chuỗi 7 ngày gần nhất từ meta_ad_insights (campaign level).
func getCampaign7DaySeries(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) ([]daySeriesRow, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return nil, nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -PredictiveTrendDays).Format("2006-01-02")

	adAccountFilter := adAccountIdFilterForMeta(adAccountId)
	filter := bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"adAccountId":         adAccountFilter,
		"ownerOrganizationId": ownerOrgID,
		"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
	}
	extractMess := bson.M{
		"$reduce": bson.M{
			"input": bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
			"initialValue": int64(0),
			"in": bson.M{
				"$add": bson.A{
					"$$value",
					bson.M{
						"$cond": bson.M{
							"if": bson.M{
								"$regexMatch": bson.M{
									"input":   bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}},
									"regex":   "messaging_conversation_started",
								},
							},
							"then": bson.M{"$convert": bson.M{"input": "$$this.value", "to": "long", "onError": 0, "onNull": 0}},
							"else": int64(0),
						},
					},
				},
			},
		},
	}
	extractFreq := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$metaData.frequency", "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractCpm := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.cpm", "$cpm"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractCtr := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.ctr", "$ctr"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractSpend := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$spend", "0"}}, "to": "double", "onError": 0, "onNull": 0}}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$addFields", Value: bson.M{
			"_mess":   extractMess,
			"_freq":   extractFreq,
			"_cpm":    extractCpm,
			"_ctr":    extractCtr,
			"_spend":  extractSpend,
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       "$dateStart",
			"frequency": bson.M{"$avg": "$_freq"},
			"cpm":       bson.M{"$avg": "$_cpm"},
			"ctr":       bson.M{"$avg": "$_ctr"},
			"spend":     bson.M{"$sum": "$_spend"},
			"mess":      bson.M{"$sum": "$_mess"},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []daySeriesRow
	for cursor.Next(ctx) {
		var doc struct {
			ID        string  `bson:"_id"`
			Frequency float64 `bson:"frequency"`
			CPM       float64 `bson:"cpm"`
			CTR       float64 `bson:"ctr"`
			Spend     float64 `bson:"spend"`
			Mess      int64   `bson:"mess"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		rows = append(rows, daySeriesRow{
			Date:      doc.ID,
			Frequency: doc.Frequency,
			CPM:       doc.CPM,
			CTR:       doc.CTR,
			Spend:     doc.Spend,
			Mess:      doc.Mess,
		})
	}
	return rows, nil
}

// RunPredictiveTrendAlerts chạy 07:45 — Linear regression 7 ngày, gửi alert khi R² ≥ 0.6 và projected hit threshold.
func RunPredictiveTrendAlerts(ctx context.Context, baseURL string) (alertsSent int, err error) {
	log := logger.GetAppLogger()
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	filter := bson.M{
		"$and": bson.A{
			bson.M{"$or": []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}}},
			adsconfig.ScopeFilterPurchaseMessaging(),
		},
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var camp struct {
			CampaignId          string             `bson:"campaignId"`
			Name                string             `bson:"name"`
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&camp); err != nil {
			continue
		}
		rows, err := getCampaign7DaySeries(ctx, camp.CampaignId, camp.AdAccountId, camp.OwnerOrganizationID)
		if err != nil || len(rows) < 5 {
			continue
		}
		n := len(rows)
		x := make([]float64, n)
		yFreq := make([]float64, n)
		yCpm := make([]float64, n)
		for i := 0; i < n; i++ {
			x[i] = float64(i)
			yFreq[i] = rows[i].Frequency
			yCpm[i] = rows[i].CPM
		}

		// Frequency Trend: projected > 3.0 trong < 7 ngày
		mF, bF, r2F := linearRegression(x, yFreq)
		if r2F >= R2MinConfidence && mF > 0 {
			projF := mF*float64(n) + bF
			if projF > 3.0 {
				daysToHit := 0
				if mF > 0 {
					daysToHit = int(math.Ceil((3.0 - bF) / mF))
					if daysToHit > 7 {
						daysToHit = 7
					}
				}
				msg := fmt.Sprintf("Freq hiện: %.2f | Sẽ hit 3.0 trong ~%d ngày", rows[n-1].Frequency, daysToHit)
				if sent, _ := SendPredictiveTrendAlert(ctx, "freq", camp.CampaignId, camp.Name, camp.AdAccountId, rows[n-1].Frequency, projF, daysToHit, msg, baseURL); sent > 0 {
					alertsSent++
				}
			}
		}

		// CPM Inflation: projected > 180k trong < 5 ngày
		mC, bC, r2C := linearRegression(x, yCpm)
		if r2C >= R2MinConfidence && mC > 0 {
			projC := mC*float64(n) + bC
			if projC > 180000 {
				daysToHit := 0
				if mC > 0 {
					daysToHit = int(math.Ceil((180000 - bC) / mC))
					if daysToHit > 5 {
						daysToHit = 5
					}
				}
				msg := fmt.Sprintf("CPM: %.0fk→%.0fk | Sẽ hit 180k trong ~%d ngày", rows[n-1].CPM/1000, projC/1000, daysToHit)
				if sent, _ := SendPredictiveTrendAlert(ctx, "cpm", camp.CampaignId, camp.Name, camp.AdAccountId, rows[n-1].CPM, projC, daysToHit, msg, baseURL); sent > 0 {
					alertsSent++
				}
			}
		}

		// CPA Mess Inflation: projected > Adaptive_Kill_Threshold trong < 5 ngày (FolkForm v4.1 Section 2.4)
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, camp.AdAccountId, camp.OwnerOrganizationID)
		killThreshold, hasAdaptive := adsadaptive.GetAdaptiveThreshold(ctx, adsconfig.KeyCpaMessKill, camp.CampaignId, camp.AdAccountId, camp.OwnerOrganizationID, cfg, time.Now())
		if !hasAdaptive {
			killThreshold = adsconfig.GetThresholdWithEventOverride(adsconfig.KeyCpaMessKill, cfg, time.Now())
		}
		yCpa := make([]float64, n)
		for i := 0; i < n; i++ {
			if rows[i].Mess > 0 {
				yCpa[i] = rows[i].Spend / float64(rows[i].Mess)
			} else {
				yCpa[i] = 0
			}
		}
		mCpa, bCpa, r2Cpa := linearRegression(x, yCpa)
		if r2Cpa >= R2MinConfidence && mCpa > 0 && killThreshold > 0 {
			projCpa := mCpa*float64(n) + bCpa
			if projCpa > killThreshold {
				daysToHit := 0
				if mCpa > 0 {
					daysToHit = int(math.Ceil((killThreshold - bCpa) / mCpa))
					if daysToHit > 5 {
						daysToHit = 5
					}
				}
				cpaNow := rows[n-1].Spend / float64(rows[n-1].Mess)
				if rows[n-1].Mess > 0 {
					msg := fmt.Sprintf("CPA: %.0fkđ→%.0fkđ | Sẽ hit kill threshold trong ~%d ngày", cpaNow/1000, projCpa/1000, daysToHit)
					if sent, _ := SendPredictiveTrendAlert(ctx, "cpa", camp.CampaignId, camp.Name, camp.AdAccountId, cpaNow, projCpa, daysToHit, msg, baseURL); sent > 0 {
						alertsSent++
					}
				}
			}
		}

		// Conv Rate Decay: CR projected < 8% trong < 7 ngày (FolkForm v4.1 Section 2.4)
		loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
		now := time.Now().In(loc)
		dateEnd := now.Format("2006-01-02")
		dateStart := now.AddDate(0, 0, -PredictiveTrendDays).Format("2006-01-02")
		ordersMap, okOrders := metasvc.GetCampaignDailyOrdersMap(ctx, camp.CampaignId, camp.AdAccountId, camp.OwnerOrganizationID, dateStart, dateEnd)
		if okOrders {
			yCr := make([]float64, n)
			validCr := 0
			for i := 0; i < n; i++ {
				orders := ordersMap[rows[i].Date]
				if rows[i].Mess > 0 && orders >= 0 {
					yCr[i] = float64(orders) / float64(rows[i].Mess) * 100 // CR %
					validCr++
				} else {
					yCr[i] = 0
				}
			}
			if validCr >= 5 {
				mCr, bCr, r2Cr := linearRegression(x, yCr)
				if r2Cr >= R2MinConfidence && mCr < 0 {
					projCr := mCr*float64(n) + bCr
					if projCr < 8 && projCr > 0 {
						daysToHit := 7
						if mCr < 0 {
							// x khi CR = 8: (8 - bCr) / mCr. Số ngày từ điểm cuối (n-1): (8-bCr)/mCr - (n-1)
							xCross := (8 - bCr) / mCr
							d := int(math.Ceil(xCross - float64(n-1)))
							if d >= 1 && d <= 7 {
								daysToHit = d
							}
						}
						crNow := yCr[n-1]
						msg := fmt.Sprintf("CR: %.1f%%→%.1f%% | Sẽ drop dưới 8%% trong ~%d ngày", crNow, projCr, daysToHit)
						if sent, _ := SendPredictiveTrendAlert(ctx, "cr_decay", camp.CampaignId, camp.Name, camp.AdAccountId, crNow, projCr, daysToHit, msg, baseURL); sent > 0 {
							alertsSent++
						}
					}
				}
			}
		}
	}

	// Account Revenue Pace: pace < 70% trước ngày 20 (FolkForm v4.1 Section 2.4)
	configColl, okCfg := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if okCfg {
		loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
		day := time.Now().In(loc).Day()
		if day < 20 {
			cur, err := configColl.Find(ctx, bson.M{"account.commonConfig.monthlyTarget": bson.M{"$gt": 0}}, mongoopts.Find().SetProjection(bson.M{"adAccountId": 1, "ownerOrganizationId": 1, "account.commonConfig.monthlyTarget": 1}))
			if err == nil && cur != nil {
				defer cur.Close(ctx)
				for cur.Next(ctx) {
					var doc struct {
						AdAccountId         string             `bson:"adAccountId"`
						OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
						Account              struct {
							CommonConfig struct {
								MonthlyTarget float64 `bson:"monthlyTarget"`
							} `bson:"commonConfig"`
						} `bson:"account"`
					}
					if cur.Decode(&doc) != nil {
						continue
					}
					monthlyTarget := doc.Account.CommonConfig.MonthlyTarget
					if monthlyTarget <= 0 {
						continue
					}
					if pace, ok := metasvc.GetMonthlyRevenuePace(ctx, doc.AdAccountId, doc.OwnerOrganizationID, monthlyTarget); ok && pace < 0.7 {
						msg := fmt.Sprintf("Revenue pace: %.0f%% | Cần tăng tốc để hit target | Xem xét BLITZ mode", pace*100)
						if sent, _ := SendPredictiveTrendPaceAlert(ctx, doc.AdAccountId, pace, monthlyTarget, msg, baseURL); sent > 0 {
							alertsSent++
						}
					}
				}
			}
		}
	}
	if alertsSent > 0 {
		log.WithFields(map[string]interface{}{"alertsSent": alertsSent}).Info("⏰ [PREDICTIVE] Đã gửi Predictive Trend Alerts")
	}
	return alertsSent, nil
}

// SendPredictiveTrendAlert gửi thông báo Predictive Trend qua notifytrigger.
func SendPredictiveTrendAlert(ctx context.Context, alertType, campaignId, campaignName, adAccountId string, currentVal, projectedVal float64, daysToHit int, message, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"alertType":     alertType,
		"campaignId":    campaignId,
		"campaignName":  campaignName,
		"adAccountId":   adAccountId,
		"currentValue": strconv.FormatFloat(currentVal, 'f', 2, 64),
		"projectedValue": strconv.FormatFloat(projectedVal, 'f', 2, 64),
		"daysToHit":    strconv.Itoa(daysToHit),
		"message":      message,
	}
	return SendAdsAlert(ctx, EventTypePredictiveTrend, payload, baseURL)
}

// SendPredictiveTrendPaceAlert gửi thông báo Account Revenue Pace (account-level).
func SendPredictiveTrendPaceAlert(ctx context.Context, adAccountId string, pace, monthlyTarget float64, message, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"alertType":     "pace",
		"adAccountId":   adAccountId,
		"pace":          strconv.FormatFloat(pace*100, 'f', 1, 64),
		"monthlyTarget": strconv.FormatFloat(monthlyTarget, 'f', 0, 64),
		"message":       message,
	}
	return SendAdsAlert(ctx, EventTypePredictiveTrend, payload, baseURL)
}
