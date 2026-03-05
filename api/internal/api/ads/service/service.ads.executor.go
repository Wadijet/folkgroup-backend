// Package adssvc — Thực thi action ads qua Meta Marketing API.
package adssvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	metaclient "meta_commerce/internal/api/meta/client"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/global"

	pkgapproval "meta_commerce/pkg/approval"
)

// ExecuteAdsAction thực thi action (KILL, PAUSE, RESUME, INCREASE, DECREASE, SET_BUDGET) qua Meta API.
func ExecuteAdsAction(ctx context.Context, doc *pkgapproval.ActionPending) (map[string]interface{}, error) {
	payload := doc.Payload
	if payload == nil {
		return nil, fmt.Errorf("payload trống")
	}
	adAccountId, _ := payload["adAccountId"].(string)
	campaignId, _ := payload["campaignId"].(string)
	adSetId, _ := payload["adSetId"].(string)
	adId, _ := payload["adId"].(string)
	value := payload["value"]

	// Lấy token từ config
	cfg := global.MongoDB_ServerConfig
	if cfg == nil {
		return nil, fmt.Errorf("chưa cấu hình server")
	}
	token := metasvc.GetEffectiveMetaToken(
		cfg.MetaAccessToken,
		cfg.MetaTokenFile,
		cfg.MetaAccessToken,
	)
	if token == "" {
		return nil, fmt.Errorf("chưa cấu hình Meta access token (META_ACCESS_TOKEN hoặc META_TOKEN_FILE)")
	}
	client := metaclient.NewMetaGraphClient(token)
	if client == nil {
		return nil, fmt.Errorf("không tạo được Meta client")
	}

	// Xác định object cần thao tác (ưu tiên ad > adset > campaign)
	objectId := adId
	objectType := "ad"
	if objectId == "" {
		objectId = adSetId
		objectType = "adset"
	}
	if objectId == "" {
		objectId = campaignId
		objectType = "campaign"
	}
	if objectId == "" {
		return nil, fmt.Errorf("payload thiếu adId, adSetId hoặc campaignId")
	}

	switch doc.ActionType {
	case "KILL", "PAUSE":
		body, err := client.Post(ctx, objectId, map[string]string{"status": "PAUSED"})
		if err != nil {
			return nil, fmt.Errorf("Meta API pause %s %s: %w", objectType, objectId, err)
		}
		return map[string]interface{}{
			"success":    true,
			"objectType": objectType,
			"objectId":   objectId,
			"status":     "PAUSED",
			"raw":        string(body),
		}, nil

	case "RESUME":
		body, err := client.Post(ctx, objectId, map[string]string{"status": "ACTIVE"})
		if err != nil {
			return nil, fmt.Errorf("Meta API resume %s %s: %w", objectType, objectId, err)
		}
		return map[string]interface{}{
			"success":    true,
			"objectType": objectType,
			"objectId":   objectId,
			"status":     "ACTIVE",
			"raw":        string(body),
		}, nil

	case "SET_BUDGET":
		// Budget phải đổi ở adset hoặc campaign. Ưu tiên adset.
		budgetObjId := adSetId
		if budgetObjId == "" {
			budgetObjId = campaignId
		}
		if budgetObjId == "" {
			return nil, fmt.Errorf("SET_BUDGET cần adSetId hoặc campaignId")
		}
		budgetCents := toBudgetCents(value)
		if budgetCents <= 0 {
			return nil, fmt.Errorf("SET_BUDGET value không hợp lệ: %v", value)
		}
		body, err := client.Post(ctx, budgetObjId, map[string]string{
			"daily_budget": strconv.FormatInt(budgetCents, 10),
		})
		if err != nil {
			return nil, fmt.Errorf("Meta API set budget %s: %w", budgetObjId, err)
		}
		return map[string]interface{}{
			"success":      true,
			"objectId":     budgetObjId,
			"dailyBudget":  budgetCents,
			"adAccountId":  adAccountId,
			"raw":          string(body),
		}, nil

	case "INCREASE", "DECREASE":
		// Cần lấy budget hiện tại rồi áp dụng %. Đơn giản: value là % (vd 15 = +15%)
		budgetObjId := adSetId
		if budgetObjId == "" {
			budgetObjId = campaignId
		}
		if budgetObjId == "" {
			return nil, fmt.Errorf("INCREASE/DECREASE cần adSetId hoặc campaignId")
		}
		// Lấy daily_budget hiện tại
		fields := "daily_budget"
		if budgetObjId == campaignId {
			fields = "daily_budget,lifetime_budget"
		}
		getBody, err := client.Get(ctx, budgetObjId, map[string]string{"fields": fields})
		if err != nil {
			return nil, fmt.Errorf("Meta API get budget %s: %w", budgetObjId, err)
		}
		var metaResp map[string]interface{}
		if err := json.Unmarshal(getBody, &metaResp); err != nil {
			return nil, fmt.Errorf("parse Meta response: %w", err)
		}
		currentBudget := extractInt64(metaResp, "daily_budget")
		if currentBudget <= 0 {
			currentBudget = extractInt64(metaResp, "lifetime_budget") / 30
		}
		if currentBudget <= 0 {
			return nil, fmt.Errorf("không lấy được budget hiện tại từ Meta")
		}
		percent := toPercent(value)
		if percent <= 0 {
			return nil, fmt.Errorf("INCREASE/DECREASE value không hợp lệ: %v", value)
		}
		var newBudget int64
		if doc.ActionType == "INCREASE" {
			newBudget = currentBudget + (currentBudget * percent / 100)
		} else {
			newBudget = currentBudget - (currentBudget * percent / 100)
		}
		if newBudget < 100 {
			newBudget = 100 // Meta minimum
		}
		body, err := client.Post(ctx, budgetObjId, map[string]string{
			"daily_budget": strconv.FormatInt(newBudget, 10),
		})
		if err != nil {
			return nil, fmt.Errorf("Meta API update budget %s: %w", budgetObjId, err)
		}
		return map[string]interface{}{
			"success":       true,
			"objectId":      budgetObjId,
			"previousBudget": currentBudget,
			"newBudget":     newBudget,
			"percent":      percent,
			"adAccountId":   adAccountId,
			"raw":           string(body),
		}, nil

	default:
		return nil, fmt.Errorf("actionType chưa hỗ trợ: %s", doc.ActionType)
	}
}

func toBudgetCents(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

func toPercent(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

func extractInt64(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}
