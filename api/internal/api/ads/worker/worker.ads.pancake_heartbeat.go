// Package worker — Pancake Heartbeat: kiểm tra pc_pos_orders có nhận dữ liệu gần đây không.
// Nếu không có order trong 2h (giờ hành chính 7-22h) → set KillRulesEnabled=false (Pancake có thể down).
package worker

import (
	"context"
	"time"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"
	coreworker "meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsPancakeHeartbeatWorker kiểm tra Pancake sync mỗi 15 phút.
type AdsPancakeHeartbeatWorker struct {
	interval time.Duration
}

// NewAdsPancakeHeartbeatWorker tạo worker mới.
func NewAdsPancakeHeartbeatWorker(interval time.Duration) *AdsPancakeHeartbeatWorker {
	if interval < 5*time.Minute {
		interval = 15 * time.Minute
	}
	return &AdsPancakeHeartbeatWorker{interval: interval}
}

// Start chạy worker.
func (w *AdsPancakeHeartbeatWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
	}).Info("💓 [PANCAKE_HEARTBEAT] Starting Pancake Heartbeat Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("💓 [PANCAKE_HEARTBEAT] Worker stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsPancakeHeartbeat) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsPancakeHeartbeat, coreworker.PriorityNormal)
			if coreworker.ShouldThrottle(p) {
				continue
			}
			if effInterval := coreworker.GetEffectiveInterval(w.interval, p); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			w.process(ctx)
		}
	}
}

func (w *AdsPancakeHeartbeatWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("💓 [PANCAKE_HEARTBEAT] Panic")
		}
	}()

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h := now.Hour()
	// Chỉ check trong giờ hành chính 7-22h
	if h < 7 || h > 22 {
		return
	}

	configColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return
	}

	// [HB-3] Divergence: check trước — FB_Mess_1h>100, Pancake_orders_1h=0, hôm qua cùng giờ có đơn (PATCH 03).
	cursor, err := configColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
			Account             struct {
				AutomationConfig struct {
					PancakeSuspectAt int64 `bson:"pancakeSuspectAt"`
				} `bson:"automationConfig"`
			} `bson:"account"`
		}
		if cursor.Decode(&doc) != nil {
			continue
		}
		mess1h, _ := metasvc.GetMessForAccountLast1h(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		orders1h, _ := metasvc.GetOrdersForAccountLast1h(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		ordersYest, _ := metasvc.GetOrdersForAccountYesterdaySameHour(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		if mess1h > 100 && orders1h == 0 && ordersYest > 0 {
			// PANCAKE_SUSPECT — freeze 60p
			_, _ = configColl.UpdateOne(ctx, bson.M{
				"adAccountId":         doc.AdAccountId,
				"ownerOrganizationId": doc.OwnerOrganizationID,
			}, bson.M{
				"$set": bson.M{
					"account.automationConfig.pancakeSuspectOverride": true,
					"account.automationConfig.pancakeSuspectAt":       time.Now().UnixMilli(),
				},
			})
			log.WithFields(map[string]interface{}{
				"adAccountId": doc.AdAccountId,
				"mess1h":      mess1h,
				"orders1h":    orders1h,
				"ordersYest":  ordersYest,
			}).Warn("💓 [PANCAKE_HEARTBEAT] [HB-3] Divergence — FB Mess 1h cao, Pancake 0 đơn, hôm qua có đơn → FREEZE 60p")
			_, _ = adssvc.SendPancakeSuspectAlert(ctx, doc.AdAccountId)
		}
		// Gỡ suspect sau 60p nếu có orders
		suspectAt := doc.Account.AutomationConfig.PancakeSuspectAt
		if suspectAt > 0 && time.Since(time.UnixMilli(suspectAt)) >= 60*time.Minute && orders1h > 0 {
			_, _ = configColl.UpdateOne(ctx, bson.M{
				"adAccountId":         doc.AdAccountId,
				"ownerOrganizationId": doc.OwnerOrganizationID,
			}, bson.M{
				"$unset": bson.M{
					"account.automationConfig.pancakeSuspectOverride": "",
					"account.automationConfig.pancakeSuspectAt":       "",
				},
			})
		}
	}

	// [HB-2] Không có order 2h — Pancake down
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return
	}
	cutoffSec := time.Now().Add(-2 * time.Hour).Unix()
	n, err := coll.CountDocuments(ctx, bson.M{"posUpdatedAt": bson.M{"$gte": cutoffSec}})
	if err != nil {
		log.WithError(err).Warn("💓 [PANCAKE_HEARTBEAT] Lỗi đếm orders")
		return
	}
	if n == 0 {
		_, err := configColl.UpdateMany(ctx, bson.M{}, bson.M{
			"$set": bson.M{
				"account.automationConfig.pancakeDownOverride": true,
				"account.automationConfig.pancakeDownAt":       time.Now().UnixMilli(),
			},
		})
		if err != nil {
			log.WithError(err).Warn("💓 [PANCAKE_HEARTBEAT] Lỗi set pancakeDownOverride")
			return
		}
		log.Warn("💓 [PANCAKE_HEARTBEAT] Không có order 2h — đã set KillRulesEnabled override (Pancake có thể down)")
		_, _ = adssvc.SendPancakeDownAlert(ctx, "")
	} else {
		_, _ = configColl.UpdateMany(ctx, bson.M{"account.automationConfig.pancakeDownOverride": true}, bson.M{
			"$unset": bson.M{
				"account.automationConfig.pancakeDownOverride": "",
				"account.automationConfig.pancakeDownAt":       "",
			},
		})
	}
}
