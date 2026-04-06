// Package adssvc — Mode Detection theo FolkForm v4.1 S03.
// Chạy 07:30 mỗi sáng: tính điểm từ signals → BLITZ | NORMAL | EFFICIENCY | PROTECT.
package adssvc

import (
	"context"
	"fmt"
	"time"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ModeBLITZ     = "BLITZ"
	ModeNORMAL    = "NORMAL"
	ModeEFFICIENCY = "EFFICIENCY"
	ModePROTECT   = "PROTECT"
)

// RunModeDetection chạy Mode Detection cho tất cả ad accounts.
// Gọi lúc 07:30 mỗi sáng. Cập nhật accountMode vào ads_meta_config (nguồn duy nhất).
func RunModeDetection(ctx context.Context) (updated int, err error) {
	log := logger.GetAppLogger()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)

	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy meta_ad_accounts")
	}
	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var accounts []struct {
		AdAccountId         string              `bson:"adAccountId"`
		OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	if err := cursor.All(ctx, &accounts); err != nil {
		return 0, err
	}

	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok || metaColl == nil {
		return 0, fmt.Errorf("không tìm thấy ads_meta_config")
	}
	for _, acc := range accounts {
		score, mode := computeModeScore(ctx, acc.AdAccountId, acc.OwnerOrganizationID, now)
		// Đảm bảo ads_meta_config tồn tại (account mới có thể chưa có config)
		adsconfig.InitDefaultConfig(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		// Cập nhật accountMode vào ads_meta_config — nguồn duy nhất
		res, err := metaColl.UpdateOne(ctx,
			bson.M{"adAccountId": acc.AdAccountId, "ownerOrganizationId": acc.OwnerOrganizationID},
			bson.M{"$set": bson.M{"account.accountMode": mode, "updatedAt": time.Now().UnixMilli()}},
		)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"adAccountId": acc.AdAccountId,
				"mode":        mode,
			}).Warn("[MODE_DETECTION] Lỗi cập nhật accountMode")
			continue
		}
		if res.MatchedCount == 0 {
			continue
		}
		updated++
		log.WithFields(map[string]interface{}{
			"adAccountId": acc.AdAccountId,
			"score":       score,
			"mode":        mode,
		}).Info("📊 [MODE_DETECTION] Đã cập nhật mode")
	}
	return updated, nil
}

// computeModeScore tính điểm từ signals S1–S5 + Event + Weekend. FolkForm v4.1 Section 3.1.
func computeModeScore(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, t time.Time) (int, string) {
	score := 3 // Base NORMAL

	// S1: ROAS Pancake hôm qua — >3x +2 BLITZ, 2–3x +1 BLITZ, <2x +1 PROTECT
	if roas, ok := metasvc.GetROASYesterday(ctx, adAccountId, ownerOrgID); ok {
		if roas > 3 {
			score += 2
		} else if roas >= 2 {
			score += 1
		} else {
			score -= 1
		}
	}

	// S2: CPM sáng 07:00–07:30 — <0.80×3day +2 BLITZ, 0.80–1.10 0, >1.30 +2 PROTECT
	if cpm0730, okCpm := metasvc.GetCPMSang0730(ctx, adAccountId, ownerOrgID); okCpm {
		if cpm3day, ok3 := metasvc.GetCPM3dayAvgFromInsights(ctx, adAccountId, ownerOrgID); ok3 && cpm3day > 0 {
			ratio := cpm0730 / cpm3day
			if ratio < 0.80 {
				score += 2
			} else if ratio > 1.30 {
				score -= 2
			}
		}
	}

	// S3: Mess velocity 07:00–07:30 — so sánh hôm nay vs hôm qua cùng giờ
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := t.In(loc)
	messToday, okToday := metasvc.GetMess0730ForDate(ctx, adAccountId, ownerOrgID, now)
	messYesterday, okYest := metasvc.GetMess0730ForDate(ctx, adAccountId, ownerOrgID, now.AddDate(0, 0, -1))
	if okToday && okYest && messYesterday > 0 {
		ratio := float64(messToday) / float64(messYesterday)
		if ratio > 1.4 {
			score += 2
		} else if ratio > 1.0 {
			score += 1
		} else if ratio < 0.6 {
			score -= 1
		}
	}

	// S4: Monthly Revenue Pace — pace < 0.80 +2 BLITZ, 0.80–1.20 0, >1.20 +1 PROTECT
	cfg, _ := adsconfig.GetConfig(ctx, adAccountId, ownerOrgID)
	monthlyTarget := 0.0
	if cfg != nil {
		monthlyTarget = cfg.Account.CommonConfig.MonthlyTarget
	}
	if pace, ok := metasvc.GetMonthlyRevenuePace(ctx, adAccountId, ownerOrgID, monthlyTarget); ok {
		if pace < 0.80 {
			score += 2
		} else if pace > 1.20 {
			score -= 1
		}
	}

	// S5: CHS Account Average — avg < 1.0 +1 BLITZ, >1.8 +1 PROTECT
	if avgChs, ok := metasvc.GetCHSAccountAvg(ctx, adAccountId, ownerOrgID); ok {
		if avgChs < 1.0 {
			score += 1
		} else if avgChs > 1.8 {
			score -= 1
		}
	}

	// Event Calendar: +3 BLITZ (override)
	if inEvent, bonus, _ := adsconfig.IsEventWindow(t); inEvent {
		score += bonus
	}
	// Weekend: -2
	if adsconfig.IsWeekend(t) {
		score -= 2
	}

	if score >= 5 {
		return score, ModeBLITZ
	}
	if score >= 3 {
		return score, ModeNORMAL
	}
	if score >= 1 {
		return score, ModeEFFICIENCY
	}
	return score, ModePROTECT
}

// GetNightOffHour trả về giờ tắt (Night Off) theo mode. FolkForm R07.
func GetNightOffHour(mode string) int {
	switch mode {
	case ModePROTECT:
		return 21
	case ModeEFFICIENCY:
		return 22
	case ModeNORMAL:
		return 22 // 22:30 — dùng 22 cho đơn giản, có thể mở rộng
	case ModeBLITZ:
		return 23
	default:
		return 22
	}
}
