// Package adssvc — Momentum Tracker theo FolkForm v4.1 S04.
// CR_now vs CR_baseline, Msg_Rate_ratio → ACCELERATING | STABLE | SLOWING | DROPPING.
package adssvc

import (
	"context"
	"time"

	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	MomentumACCELERATING = "ACCELERATING"
	MomentumSTABLE       = "STABLE"
	MomentumSLOWING      = "SLOWING"
	MomentumDROPPING     = "DROPPING"
)

// RunMomentumTracking cập nhật momentumState cho từng ad account. Chạy mỗi 30p.
// Lưu momentumState, momentumCheckpointCount vào meta_ad_accounts.
func RunMomentumTracking(ctx context.Context) (updated int, err error) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return 0, nil
	}
	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId         string                 `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID    `bson:"ownerOrganizationId"`
			CurrentMetrics      map[string]interface{} `bson:"currentMetrics"`
			MomentumState       string                `bson:"momentumState"`
			MomentumCheckpoint  int                   `bson:"momentumCheckpoint"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		state, checkpoint := computeMomentumState(doc.CurrentMetrics, doc.MomentumState, doc.MomentumCheckpoint)
		if state == "" {
			continue
		}
		_, err := accColl.UpdateOne(ctx,
			bson.M{"adAccountId": doc.AdAccountId, "ownerOrganizationId": doc.OwnerOrganizationID},
			bson.M{"$set": bson.M{
				"momentumState":      state,
				"momentumCheckpoint": checkpoint,
				"momentumUpdatedAt":  time.Now().UnixMilli(),
			}},
		)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{"adAccountId": doc.AdAccountId}).Warn("[MOMENTUM] Lỗi cập nhật")
			continue
		}
		updated++
	}
	return updated, nil
}

// computeMomentumState tính state từ currentMetrics. Trả về (state, checkpoint).
func computeMomentumState(current map[string]interface{}, prevState string, prevCheckpoint int) (string, int) {
	if current == nil {
		return "", 0
	}
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		return "", 0
	}
	r7d, _ := raw["7d"].(map[string]interface{})
	r2h, _ := raw["2h"].(map[string]interface{})
	r30p, _ := raw["30p"].(map[string]interface{})

	// CR_baseline = orders_7d / mess_7d
	meta7d, _ := r7d["meta"].(map[string]interface{})
	pancake7d, _ := r7d["pancake"].(map[string]interface{})
	pos7d, _ := pancake7d["pos"].(map[string]interface{})
	orders7d := toFloatFromMap(pos7d, "orders")
	mess7d := toFloatFromMap(meta7d, "mess")
	crBaseline := 0.0
	if mess7d > 0 {
		crBaseline = orders7d / mess7d
	}

	// CR_now = orders_2h / mess_2h
	orders2h := toFloatFromMap(r2h, "orders")
	mess2h := toFloatFromMap(r2h, "mess")
	crNow := 0.0
	if mess2h > 0 {
		crNow = orders2h / mess2h
	}

	// Msg_Rate_ratio — đơn giản: mess_30p / clicks. Nếu không có 30p thì bỏ qua.
	msgRateRatio := 1.0
	if r30p != nil {
		mess30p := toFloatFromMap(r30p, "mess")
		clicks30p := toFloatFromMap(r30p, "clicks")
		if clicks30p > 0 {
			msgRateNow := (mess30p / clicks30p) * 100
			// Msg_Rate_3day_avg — tạm dùng 8% làm baseline
			msgRateRatio = msgRateNow / 8.0
		}
	}

	// Quyết định state
	if crBaseline <= 0 {
		return MomentumSTABLE, 0
	}
	ratio := crNow / crBaseline

	var state string
	checkpoint := prevCheckpoint
	if ratio > 1.3 {
		state = MomentumACCELERATING
		if prevState == MomentumACCELERATING {
			checkpoint = prevCheckpoint + 1
		} else {
			checkpoint = 1
		}
	} else if ratio >= 0.9 && ratio <= 1.3 && msgRateRatio > 0.8 {
		state = MomentumSTABLE
		checkpoint = 0
	} else if ratio >= 0.7 && ratio < 0.9 || msgRateRatio < 0.6 {
		state = MomentumSLOWING
		if prevState == MomentumSLOWING {
			checkpoint = prevCheckpoint + 1
		} else {
			checkpoint = 1
		}
	} else if ratio < 0.7 || msgRateRatio < 0.5 {
		state = MomentumDROPPING
		checkpoint = 1
	} else {
		state = MomentumSTABLE
		checkpoint = 0
	}
	return state, checkpoint
}

func toFloatFromMap(m map[string]interface{}, k string) float64 {
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
