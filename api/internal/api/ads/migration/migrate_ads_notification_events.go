// Package migration — Init templates và routing rules cho ads notification events.
// Gọi khi init để thông báo FolkForm (Circuit Breaker, Pancake Down, v.v.) gửi qua hệ thống notification.
package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	authmodels "meta_commerce/internal/api/auth/models"
	authsvc "meta_commerce/internal/api/auth/service"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
)

// adsEvent định nghĩa event type ads cho notification.
type adsEvent struct {
	eventType string
	subject   string
	content   string
	variables []string
}

// InitAdsNotificationEvents khởi tạo templates và routing rules cho ads events.
// Gửi thông báo đến System Organization (Telegram, email, webhook).
// Gọi một lần khi init; bỏ qua nếu đã có.
func InitAdsNotificationEvents(ctx context.Context) (int, error) {
	ctx = basesvc.WithSystemDataInsertAllowed(ctx)
	log := logger.GetAppLogger()

	systemOrg, err := getSystemOrganization(ctx)
	if err != nil {
		return 0, fmt.Errorf("lấy System Organization: %w", err)
	}

	adsEvents := []adsEvent{
		{
			eventType: "ads_circuit_breaker_alert",
			subject:   "🚨 [ADS] Circuit Breaker — Đã PAUSE toàn account",
			content: `Cảnh báo: Circuit Breaker đã kích hoạt.

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Mã trigger: {{code}}
- Chi tiết: {{message}}

Tất cả campaign đã được PAUSE. Cần xác nhận /resume_ads sau khi kiểm tra.

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "code", "message"},
		},
		{
			eventType: "ads_pancake_down",
			subject:   "⚠️ [ADS] Pancake có thể down — Không có order 2h",
			content: `Cảnh báo: Không có đơn Pancake trong 2 giờ qua (giờ hành chính).

Thông tin:
- Thời gian: {{timestamp}}
- Hành động: Đã tắt Kill Rules (pancakeDownOverride) để tránh kill nhầm.

Vui lòng kiểm tra Pancake POS sync. Gọi /pancake_ok khi đã xác nhận.

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp"},
		},
		{
			eventType: "ads_momentum_alert",
			subject:   "📊 [ADS] Momentum Tracker — Trạng thái thay đổi",
			content: `Momentum Tracker phát hiện thay đổi.

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Trạng thái: {{momentum}}
- Conv Rate 2h: {{convRate2h}}%

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "momentum", "convRate2h"},
		},
		{
			eventType: "ads_mode_detected",
			subject:   "📋 [ADS] Mode Detection — Chế độ ngày",
			content: `Mode Detection đã chạy (07:30).

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Mode: {{mode}}
- Điểm: {{score}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "mode", "score"},
		},
		{
			eventType: "ads_night_off",
			subject:   "🌙 [ADS] Night Off — Đã tắt quảng cáo",
			content: `Night Off đã kích hoạt (21h–23h).

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Campaigns đã pause: {{count}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "count"},
		},
		{
			eventType: "ads_noon_cut",
			subject:   "☀️ [ADS] Noon Cut — Cắt budget trưa",
			content: `Noon Cut Off đã chạy (12:30/14:00).

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Chi tiết: {{message}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "message"},
		},
		{
			eventType: "ads_morning_on",
			subject:   "🌅 [ADS] Morning On — Bật quảng cáo sáng",
			content: `Morning On đã chạy (06:00).

Thông tin:
- Thời gian: {{timestamp}}
- Ad Account: {{adAccountId}}
- Campaigns đã resume: {{count}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "adAccountId", "count"},
		},
		{
			eventType: "ads_chs_kill",
			subject:   "📊 [ADS] CHS Kill — Camp Health Score > 2.0",
			content: `CHS Kill: Camp bị kill do Camp Health Score > 2.0.

Thông tin:
- Thời gian: {{timestamp}}
- Campaign: {{campaignId}}
- CHS: {{chs}}
- Chi tiết: {{message}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "campaignId", "chs", "message"},
		},
		{
			eventType: "ads_action_pending_approval",
			subject:   "📢 [ADS] Đề xuất chờ duyệt — {{actionType}}",
			content: `Có đề xuất mới cần duyệt.

▸ Thông tin đề xuất
- Thời gian: {{timestamp}}
- Hành động: {{actionType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}
- Rule: {{ruleCode}}
- Lý do: {{reason}}

▸ Căn cứ tạo đề xuất

1. Flags trigger: {{flagsSummary}}

2. Dữ liệu Raw (7d): {{rawSummary}}

3. Chỉ số Layer1: {{layer1Summary}}

4. Layer3: {{layer3Summary}}

5. Chi tiết từng điều kiện (giá trị vs ngưỡng):
{{flagsDetail}}

▸ Hành động
Link duyệt: {{approveUrl}}
Link từ chối: {{rejectUrl}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "actionType", "campaignName", "campaignId", "adAccountId", "ruleCode", "reason", "flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail", "approveUrl", "rejectUrl"},
		},
		{
			eventType: "ads_action_executed",
			subject:   "✅ [ADS] Đã thực thi thành công — {{actionType}}",
			content: `Đề xuất đã được thực thi thành công qua Meta API.

▸ Thông tin
- Thời gian: {{timestamp}}
- Hành động: {{actionType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}
- Rule: {{ruleCode}}

▸ Căn cứ đã thực thi
- Flags: {{flagsSummary}}
- Raw: {{rawSummary}}
- Layer1: {{layer1Summary}}
- Layer3: {{layer3Summary}}
- Chi tiết: {{flagsDetail}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "actionType", "campaignName", "campaignId", "adAccountId", "ruleCode", "flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail"},
		},
		{
			eventType: "ads_action_rejected",
			subject:   "❌ [ADS] Đề xuất bị từ chối — {{actionType}}",
			content: `Đề xuất đã bị từ chối bởi người duyệt.

▸ Thông tin
- Thời gian: {{timestamp}}
- Hành động: {{actionType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}
- Người từ chối: {{rejectedBy}}
- Lý do: {{reason}}

▸ Căn cứ đề xuất (đã từ chối)
- Flags: {{flagsSummary}}
- Raw: {{rawSummary}}
- Layer1: {{layer1Summary}}
- Layer3: {{layer3Summary}}
- Chi tiết: {{flagsDetail}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "actionType", "campaignName", "campaignId", "adAccountId", "rejectedBy", "reason", "flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail"},
		},
		{
			eventType: "ads_action_executed_failed",
			subject:   "🚨 [ADS] Thực thi thất bại — {{actionType}}",
			content: `Đề xuất thực thi qua Meta API thất bại sau nhiều lần retry.

▸ Thông tin
- Thời gian: {{timestamp}}
- Hành động: {{actionType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}
- Lỗi: {{executeError}}

▸ Căn cứ đề xuất
- Flags: {{flagsSummary}}
- Raw: {{rawSummary}}
- Layer1: {{layer1Summary}}
- Layer3: {{layer3Summary}}
- Chi tiết: {{flagsDetail}}

Vui lòng kiểm tra và xử lý thủ công nếu cần.

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "actionType", "campaignName", "campaignId", "adAccountId", "executeError", "flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail"},
		},
		{
			eventType: "ads_action_cancelled",
			subject:   "🚫 [ADS] Đề xuất đã hủy — {{actionType}}",
			content: `Đề xuất đã bị hủy trước khi duyệt.

▸ Thông tin
- Thời gian: {{timestamp}}
- Hành động: {{actionType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}

▸ Căn cứ đề xuất (đã hủy)
- Flags: {{flagsSummary}}
- Raw: {{rawSummary}}
- Layer1: {{layer1Summary}}
- Layer3: {{layer3Summary}}
- Chi tiết: {{flagsDetail}}

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "actionType", "campaignName", "campaignId", "adAccountId", "flagsSummary", "rawSummary", "layer1Summary", "layer3Summary", "flagsDetail"},
		},
		{
			eventType: "ads_predictive_trend_alert",
			subject:   "⏰ [ADS] Predictive Trend — {{alertType}}",
			content: `Cảnh báo dự báo xu hướng (Linear regression 7 ngày, R² ≥ 0.6).

Thông tin:
- Thời gian: {{timestamp}}
- Loại: {{alertType}}
- Campaign: {{campaignName}} ({{campaignId}})
- Ad Account: {{adAccountId}}
- Giá trị hiện tại: {{currentValue}}
- Dự báo: {{projectedValue}}
- Sẽ hit ngưỡng trong ~{{daysToHit}} ngày
- Chi tiết: {{message}}

Chuẩn bị creative mới / Review audience / Kiểm tra overlap nếu cần.

Trân trọng,
Hệ thống Ads`,
			variables: []string{"timestamp", "alertType", "campaignName", "campaignId", "adAccountId", "currentValue", "projectedValue", "daysToHit", "message"},
		},
	}

	templateService, err := notifsvc.NewNotificationTemplateService()
	if err != nil {
		return 0, fmt.Errorf("tạo template service: %w", err)
	}
	// Routing: dùng domain rules (domain=ads → Marketing Team), không tạo rule theo eventType nữa

	currentTime := time.Now().Unix()
	created := 0

	for _, event := range adsEvents {
		// Templates (email, telegram, webhook)
		for _, channelType := range []string{"email", "telegram", "webhook"} {
			filter := bson.M{
				"ownerOrganizationId": systemOrg.ID,
				"eventType":           event.eventType,
				"channelType":         channelType,
			}
			_, err := templateService.FindOne(ctx, filter, nil)
			if err == common.ErrNotFound {
				tpl := notifmodels.NotificationTemplate{
					OwnerOrganizationID: &systemOrg.ID,
					EventType:           event.eventType,
					ChannelType:         channelType,
					Description:         fmt.Sprintf("Template %s cho event '%s'. FolkForm Ads.", channelType, event.eventType),
					Subject:             event.subject,
					Content:             event.content,
					Variables:           event.variables,
					IsActive:            true,
					IsSystem:            true,
					CreatedAt:           currentTime,
					UpdatedAt:           currentTime,
				}
				if channelType == "telegram" {
					tpl.Subject = ""
					tpl.Content = fmt.Sprintf("*%s*\n\n%s", event.subject, strings.ReplaceAll(event.content, "- ", "• "))
				}
				if channelType == "webhook" {
					tpl.Subject = ""
					jsonVars := make([]string, 0, len(event.variables))
					for _, v := range event.variables {
						jsonVars = append(jsonVars, fmt.Sprintf(`"%s":"{{%s}}"`, v, v))
					}
					tpl.Content = fmt.Sprintf(`{"eventType":"%s",%s}`, event.eventType, strings.Join(jsonVars, ","))
				}
				_, err = templateService.InsertOne(ctx, tpl)
				if err != nil {
					log.WithError(err).WithField("eventType", event.eventType).Warn("[ADS_INIT] Lỗi tạo template")
					continue
				}
				created++
			} else if err == nil && isAdsActionEvent(event.eventType) {
				// Cập nhật template ads_action_* đã tồn tại — bổ sung căn cứ (raw, layer, flags) vào nội dung
				subject := event.subject
				content := event.content
				if channelType == "telegram" {
					subject = ""
					content = fmt.Sprintf("*%s*\n\n%s", event.subject, strings.ReplaceAll(event.content, "- ", "• "))
				}
				if channelType == "webhook" {
					subject = ""
					jsonVars := make([]string, 0, len(event.variables))
					for _, v := range event.variables {
						jsonVars = append(jsonVars, fmt.Sprintf(`"%s":"{{%s}}"`, v, v))
					}
					content = fmt.Sprintf(`{"eventType":"%s",%s}`, event.eventType, strings.Join(jsonVars, ","))
				}
				updateData := map[string]interface{}{"subject": subject, "content": content, "variables": event.variables}
				// Dùng UpdateMany: không trả lỗi khi ModifiedCount=0 (template đã đúng)
				if n, updErr := templateService.UpdateMany(ctx, filter, updateData, nil); updErr == nil {
					if n > 0 {
						created++
						log.WithField("eventType", event.eventType).WithField("channelType", channelType).Info("[ADS_INIT] Đã cập nhật template với căn cứ đề xuất")
					}
				}
			}
		}
	}

	return created, nil
}

// isAdsActionEvent kiểm tra eventType có phải ads action (pending, executed, rejected, failed, cancelled) không.
func isAdsActionEvent(eventType string) bool {
	switch eventType {
	case "ads_action_pending_approval", "ads_action_executed", "ads_action_rejected",
		"ads_action_executed_failed", "ads_action_cancelled":
		return true
	}
	return false
}

func getSystemOrganization(ctx context.Context) (*authmodels.Organization, error) {
	orgService, err := authsvc.NewOrganizationService()
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"level": -1,
		"code":  "SYSTEM",
		"type":  authmodels.OrganizationTypeSystem,
	}
	org, err := orgService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	return &org, nil
}
