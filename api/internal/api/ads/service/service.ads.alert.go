// Package adssvc — Gửi thông báo ads qua hệ thống notification (notifytrigger).
// Dùng cho Circuit Breaker, Pancake Down, Momentum, Mode Detection, v.v.
package adssvc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"meta_commerce/internal/cta"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notifytrigger"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventType ads notification.
const (
	EventTypeCircuitBreaker  = "ads_circuit_breaker_alert"
	EventTypePancakeDown     = "ads_pancake_down"
	EventTypePancakeSuspect  = "ads_pancake_suspect" // [HB-3] Divergence: FB Mess cao, Pancake 0 đơn
	EventTypeMomentum      = "ads_momentum_alert"
	EventTypeModeDetected  = "ads_mode_detected"
	EventTypeNightOff      = "ads_night_off"
	EventTypeNoonCut       = "ads_noon_cut"
	EventTypeMorningOn     = "ads_morning_on"
	EventTypeCHSKill       = "ads_chs_kill"
	EventTypePredictiveTrend = "ads_predictive_trend_alert"
)

// SendAdsAlert gửi thông báo ads qua notifytrigger.
// Dùng System Organization để nhận (routing rules đã cấu hình).
//
// Tham số:
//   - ctx: context
//   - eventType: loại sự kiện (vd: EventTypeCircuitBreaker)
//   - payload: biến render template (timestamp, adAccountId, code, message, ...)
//   - baseURL: base URL cho CTA (rỗng = lấy từ env BASE_URL)
//
// Trả về: số item đã enqueue, error
func SendAdsAlert(ctx context.Context, eventType string, payload map[string]interface{}, baseURL string) (int, error) {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	if _, ok := payload["timestamp"]; !ok {
		payload["timestamp"] = time.Now().Format(time.RFC3339)
	}
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return 0, fmt.Errorf("lấy System Organization: %w", err)
	}
	if baseURL == "" {
		baseURL = os.Getenv("BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	count, err := notifytrigger.TriggerProgrammatic(ctx, eventType, payload, systemOrgID, baseURL)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"eventType": eventType,
		}).Warn("🔔 [ADS_ALERT] Lỗi gửi thông báo")
		return 0, err
	}
	if count > 0 {
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"eventType": eventType,
			"queued":    count,
		}).Info("🔔 [ADS_ALERT] Đã gửi thông báo")
	}
	return count, nil
}

// SendCircuitBreakerAlert gửi thông báo khi Circuit Breaker kích hoạt.
func SendCircuitBreakerAlert(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, code, message, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"adAccountId": adAccountId,
		"ownerOrgId":  ownerOrgID.Hex(),
		"code":       code,
		"message":    message,
	}
	return SendAdsAlert(ctx, EventTypeCircuitBreaker, payload, baseURL)
}

// SendPancakeDownAlert gửi thông báo khi Pancake có thể down (không có order 2h).
func SendPancakeDownAlert(ctx context.Context, baseURL string) (int, error) {
	return SendAdsAlert(ctx, EventTypePancakeDown, nil, baseURL)
}

// SendPancakeSuspectAlert gửi thông báo khi [HB-3] Divergence: FB Mess 1h>100, Pancake 0 đơn, hôm qua có đơn → freeze 60p.
func SendPancakeSuspectAlert(ctx context.Context, adAccountId string) (int, error) {
	payload := map[string]interface{}{"adAccountId": adAccountId}
	return SendAdsAlert(ctx, EventTypePancakeSuspect, payload, "")
}

// SendMomentumAlert gửi thông báo khi Momentum Tracker phát hiện thay đổi.
func SendMomentumAlert(ctx context.Context, adAccountId string, momentum string, convRate2h float64, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"adAccountId":  adAccountId,
		"momentum":     momentum,
		"convRate2h":   strconv.FormatFloat(convRate2h, 'f', 2, 64),
	}
	return SendAdsAlert(ctx, EventTypeMomentum, payload, baseURL)
}

// SendModeDetectedAlert gửi thông báo khi Mode Detection chạy.
func SendModeDetectedAlert(ctx context.Context, adAccountId string, mode string, score int, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"adAccountId": adAccountId,
		"mode":        mode,
		"score":       strconv.Itoa(score),
	}
	return SendAdsAlert(ctx, EventTypeModeDetected, payload, baseURL)
}

// SendCHSKillAlert gửi thông báo khi CHS Kill trigger.
func SendCHSKillAlert(ctx context.Context, campaignId string, chs float64, message, baseURL string) (int, error) {
	payload := map[string]interface{}{
		"campaignId": campaignId,
		"chs":        strconv.FormatFloat(chs, 'f', 2, 64),
		"message":    message,
	}
	return SendAdsAlert(ctx, EventTypeCHSKill, payload, baseURL)
}
