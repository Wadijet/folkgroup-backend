// Script chẩn đoán tại sao Auto-Propose không tạo action.
// Kiểm tra chuỗi điều kiện: ads_meta_config → meta_campaigns → action_pending_approval.
//
// Chạy: cd api && go run ../scripts/diagnose_auto_propose.go
// Output: scripts/reports/BAO_CAO_CHAN_DOAN_AUTO_PROPOSE_YYYYMMDD.md
package main

import (
	"context"
	"fmt"
	"log"
	"meta_commerce/config"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)

	var sb strings.Builder
	now := time.Now().Format("2006-01-02 15:04")
	reportDate := time.Now().Format("20060102")

	sb.WriteString("# BÁO CÁO CHẨN ĐOÁN AUTO-PROPOSE\n\n")
	sb.WriteString(fmt.Sprintf("**Ngày tạo:** %s  \n**Database:** %s\n\n", now, cfg.MongoDB_DBName_Auth))
	sb.WriteString("---\n\n")

	// 1. ads_meta_config
	writeAdsMetaConfigSection(&sb, db, ctx)

	// 2. meta_campaigns
	writeMetaCampaignsSection(&sb, db, ctx)

	// 3. Chuỗi điều kiện GetCampaignsForAutoPropose
	writeAutoProposeChainSection(&sb, db, ctx)

	// 4. action_pending_approval
	writeActionPendingSection(&sb, db, ctx)

	// 5. Kết luận và gợi ý
	writeConclusionSection(&sb, db, ctx)

	// Ghi file
	reportDir := filepath.Join("..", "scripts", "reports")
	if wd, _ := os.Getwd(); !strings.Contains(wd, "api") {
		reportDir = filepath.Join("scripts", "reports")
	}
	_ = os.MkdirAll(reportDir, 0755)
	outPath := filepath.Join(reportDir, fmt.Sprintf("BAO_CAO_CHAN_DOAN_AUTO_PROPOSE_%s.md", reportDate))
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("Ghi file: %v", err)
	}
	fmt.Printf("✅ Đã tạo báo cáo: %s\n", outPath)
}

func writeAdsMetaConfigSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 1. ADS_META_CONFIG\n\n")
	coll := db.Collection("ads_meta_config")

	total, _ := coll.CountDocuments(ctx, bson.M{})
	sb.WriteString(fmt.Sprintf("Tổng documents: **%d**\n\n", total))

	if total == 0 {
		sb.WriteString("⚠️ **VẤN ĐỀ:** Không có ads_meta_config nào. Auto-propose cần ít nhất 1 config với `account.automationConfig.autoProposeEnabled = true` (hoặc không set).\n\n")
		return
	}

	// Configs có autoProposeEnabled = true hoặc không set
	configsWithAutoPropose, _ := coll.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"account.automationConfig.autoProposeEnabled": true},
			{"account.automationConfig.autoProposeEnabled": bson.M{"$exists": false}},
		},
	})
	sb.WriteString(fmt.Sprintf("Configs có autoProposeEnabled (true hoặc không set): **%d**\n\n", configsWithAutoPropose))

	// Liệt kê chi tiết
	cur, err := coll.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"adAccountId": 1, "ownerOrganizationId": 1,
		"account.automationConfig.autoProposeEnabled": 1,
	}))
	if err != nil {
		sb.WriteString(fmt.Sprintf("Lỗi query: %v\n\n", err))
		return
	}
	defer cur.Close(ctx)

	sb.WriteString("| adAccountId | ownerOrganizationId | autoProposeEnabled |\n|-------------|---------------------|--------------------|\n")
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		accId := getStr(doc, "adAccountId")
		orgId := ""
		if oid, ok := doc["ownerOrganizationId"].(primitive.ObjectID); ok {
			orgId = oid.Hex()
		}
		autoPropose := "—"
		if acc, ok := doc["account"].(bson.M); ok {
			if aut, ok := acc["automationConfig"].(bson.M); ok {
				if v, ok := aut["autoProposeEnabled"].(bool); ok {
					autoPropose = fmt.Sprintf("%v", v)
				}
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", accId, orgId, autoPropose))
	}
	sb.WriteString("\n")
}

func writeMetaCampaignsSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 2. META_CAMPAIGNS\n\n")
	coll := db.Collection("meta_campaigns")

	total, _ := coll.CountDocuments(ctx, bson.M{})
	sb.WriteString(fmt.Sprintf("Tổng campaigns: **%d**\n\n", total))

	if total == 0 {
		sb.WriteString("⚠️ **VẤN ĐỀ:** Không có campaign nào trong meta_campaigns.\n\n")
		return
	}

	// Campaigns ACTIVE
	activeCount, _ := coll.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"effectiveStatus": "ACTIVE"},
			{"status": "ACTIVE"},
		},
	})
	sb.WriteString(fmt.Sprintf("Campaigns ACTIVE (effectiveStatus hoặc status): **%d**\n\n", activeCount))

	// Campaigns có currentMetrics.alertFlags
	withAlertFlags, _ := coll.CountDocuments(ctx, bson.M{
		"currentMetrics.alertFlags.0": bson.M{"$exists": true},
	})
	sb.WriteString(fmt.Sprintf("Campaigns có currentMetrics.alertFlags (ít nhất 1 flag): **%d**\n\n", withAlertFlags))

	// Mẫu campaigns có alertFlags
	cur, err := coll.Find(ctx, bson.M{"currentMetrics.alertFlags.0": bson.M{"$exists": true}},
		options.Find().SetLimit(10).SetProjection(bson.M{
			"campaignId": 1, "adAccountId": 1, "name": 1, "ownerOrganizationId": 1,
			"effectiveStatus": 1, "status": 1, "currentMetrics.alertFlags": 1,
		}))
	if err != nil {
		sb.WriteString(fmt.Sprintf("Lỗi query: %v\n\n", err))
		return
	}
	defer cur.Close(ctx)

	sb.WriteString("### Mẫu campaigns có alertFlags (tối đa 10)\n\n")
	sb.WriteString("| campaignId | adAccountId | name | ownerOrgId | status | alertFlags |\n")
	sb.WriteString("|------------|-------------|------|------------|--------|------------|\n")
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		campId := getStr(doc, "campaignId")
		accId := getStr(doc, "adAccountId")
		name := getStr(doc, "name")
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		orgId := ""
		if oid, ok := doc["ownerOrganizationId"].(primitive.ObjectID); ok {
			orgId = oid.Hex()[:8] + "..."
		}
		effStatus := getStr(doc, "effectiveStatus")
		status := getStr(doc, "status")
		st := effStatus
		if st == "" {
			st = status
		}
		flags := "[]"
		if cm, ok := doc["currentMetrics"].(bson.M); ok {
			if af, ok := cm["alertFlags"]; ok {
				flags = fmt.Sprintf("%v", af)
				if len(flags) > 40 {
					flags = flags[:40] + "..."
				}
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", campId, accId, name, orgId, st, flags))
	}
	sb.WriteString("\n")
}

func writeAutoProposeChainSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 3. CHUỖI ĐIỀU KIỆN GetCampaignsForAutoPropose\n\n")

	configColl := db.Collection("ads_meta_config")
	campaignsColl := db.Collection("meta_campaigns")

	// Bước 1: Lấy ad accounts có autoProposeEnabled
	configCur, err := configColl.Find(ctx, bson.M{
		"$or": []bson.M{
			{"account.automationConfig.autoProposeEnabled": true},
			{"account.automationConfig.autoProposeEnabled": bson.M{"$exists": false}},
		},
	}, options.Find().SetProjection(bson.M{"adAccountId": 1, "ownerOrganizationId": 1}))
	if err != nil {
		sb.WriteString(fmt.Sprintf("Lỗi bước 1: %v\n\n", err))
		return
	}
	defer configCur.Close(ctx)

	var configs []struct {
		AdAccountId         string              `bson:"adAccountId"`
		OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	if err := configCur.All(ctx, &configs); err != nil {
		sb.WriteString(fmt.Sprintf("Lỗi decode configs: %v\n\n", err))
		return
	}

	sb.WriteString(fmt.Sprintf("**Bước 1:** Configs có autoProposeEnabled: **%d**\n\n", len(configs)))

	if len(configs) == 0 {
		sb.WriteString("❌ **DỪNG:** Không có config nào. Kiểm tra ads_meta_config có `account.automationConfig.autoProposeEnabled = true` hoặc field không tồn tại.\n\n")
		return
	}

	rawIds := make([]string, 0, len(configs))
	orgIds := make([]primitive.ObjectID, 0, len(configs))
	for _, c := range configs {
		rawIds = append(rawIds, c.AdAccountId)
		orgIds = append(orgIds, c.OwnerOrganizationID)
	}
	adAccountIds := expandAdAccountIds(rawIds)

	// Bước 2: Query campaigns theo filter giống GetCampaignsForAutoPropose
	filter := bson.M{
		"adAccountId": bson.M{"$in": adAccountIds},
		"ownerOrganizationId": bson.M{"$in": orgIds},
		"$or": []bson.M{
			{"effectiveStatus": "ACTIVE"},
			{"status": "ACTIVE"},
		},
		"currentMetrics.alertFlags.0": bson.M{"$exists": true},
	}

	eligibleCount, err := campaignsColl.CountDocuments(ctx, filter)
	if err != nil {
		sb.WriteString(fmt.Sprintf("Lỗi bước 2: %v\n\n", err))
		return
	}

	sb.WriteString(fmt.Sprintf("**Bước 2:** Campaigns thỏa TẤT CẢ điều kiện (adAccountId in configs, ownerOrgId in configs, ACTIVE, có alertFlags): **%d**\n\n", eligibleCount))

	if eligibleCount == 0 {
		sb.WriteString("❌ **NGUYÊN NHÂN CÓ THỂ:**\n")
		sb.WriteString("- meta_campaigns.adAccountId hoặc ownerOrganizationId **không khớp** với ads_meta_config\n")
		sb.WriteString("- Campaigns không có status/effectiveStatus = ACTIVE\n")
		sb.WriteString("- Campaigns **chưa có currentMetrics.alertFlags** (cần chạy Recalculate để tính layer3 + alertFlags)\n\n")
		return
	}

	sb.WriteString("✅ Có campaigns đủ điều kiện để auto-propose. Nếu vẫn không tạo action, kiểm tra:\n")
	sb.WriteString("- Rule evaluation: alertFlags có trigger rule nào không (Kill/Decrease/Increase)\n")
	sb.WriteString("- ShouldAutoPropose: rule có autoPropose = true trong ActionRuleConfig\n")
	sb.WriteString("- HasPendingProposalForCampaign: đã có pending cho campaign chưa\n\n")
}

func writeActionPendingSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 4. ACTION_PENDING_APPROVAL\n\n")
	coll := db.Collection("action_pending_approval")

	total, _ := coll.CountDocuments(ctx, bson.M{})
	domainAds, _ := coll.CountDocuments(ctx, bson.M{"domain": "ads"})
	pending, _ := coll.CountDocuments(ctx, bson.M{"domain": "ads", "status": "pending"})

	sb.WriteString("| Loại | Số lượng |\n|------|----------|\n")
	sb.WriteString(fmt.Sprintf("| Tổng | %d |\n", total))
	sb.WriteString(fmt.Sprintf("| domain=ads | %d |\n", domainAds))
	sb.WriteString(fmt.Sprintf("| domain=ads, status=pending | %d |\n", pending))
	sb.WriteString("\n")
}

func writeConclusionSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 5. KẾT LUẬN VÀ GỢI Ý\n\n")

	configColl := db.Collection("ads_meta_config")
	campaignsColl := db.Collection("meta_campaigns")

	configCount, _ := configColl.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"account.automationConfig.autoProposeEnabled": true},
			{"account.automationConfig.autoProposeEnabled": bson.M{"$exists": false}},
		},
	})

	// Campaigns có alertFlags nhưng KHÔNG nằm trong config
	campWithFlags, _ := campaignsColl.CountDocuments(ctx, bson.M{"currentMetrics.alertFlags.0": bson.M{"$exists": true}})

	// Kiểm tra adAccountId/ownerOrgId có khớp không
	configCur, _ := configColl.Find(ctx, bson.M{
		"$or": []bson.M{
			{"account.automationConfig.autoProposeEnabled": true},
			{"account.automationConfig.autoProposeEnabled": bson.M{"$exists": false}},
		},
	}, options.Find().SetProjection(bson.M{"adAccountId": 1, "ownerOrganizationId": 1}))
	var configs []struct {
		AdAccountId         string              `bson:"adAccountId"`
		OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	_ = configCur.All(ctx, &configs)
	configCur.Close(ctx)

	// Mở rộng adAccountIds để match cả "act_XXX" và "XXX" (meta_campaigns có thể lưu cả hai format)
	rawIds := make([]string, 0)
	for _, c := range configs {
		rawIds = append(rawIds, c.AdAccountId)
	}
	adAccountIds := expandAdAccountIds(rawIds)
	orgIds := make([]primitive.ObjectID, 0)
	for _, c := range configs {
		orgIds = append(orgIds, c.OwnerOrganizationID)
	}

	eligibleCount := int64(0)
	if len(adAccountIds) > 0 {
		eligibleCount, _ = campaignsColl.CountDocuments(ctx, bson.M{
			"adAccountId":         bson.M{"$in": adAccountIds},
			"ownerOrganizationId": bson.M{"$in": orgIds},
			"$or": []bson.M{
				{"effectiveStatus": "ACTIVE"},
				{"status": "ACTIVE"},
			},
			"currentMetrics.alertFlags.0": bson.M{"$exists": true},
		})
	}

	sb.WriteString("### Checklist\n\n")
	sb.WriteString(fmt.Sprintf("- [ ] ads_meta_config có config với autoProposeEnabled: **%d** configs\n", configCount))
	sb.WriteString(fmt.Sprintf("- [ ] meta_campaigns có alertFlags: **%d** campaigns\n", campWithFlags))
	sb.WriteString(fmt.Sprintf("- [ ] Campaigns đủ điều kiện (config + ACTIVE + alertFlags): **%d**\n\n", eligibleCount))

	if configCount == 0 {
		sb.WriteString("### 🔧 Hành động đề xuất\n\n")
		sb.WriteString("1. **Tạo ads_meta_config** cho ad account: Gọi API cấu hình Meta Ads, đảm bảo `account.automationConfig.autoProposeEnabled = true` (hoặc không set, mặc định true).\n\n")
	} else if campWithFlags == 0 {
		sb.WriteString("### 🔧 Hành động đề xuất\n\n")
		sb.WriteString("1. **Chạy Recalculate** cho campaigns: Campaigns cần có `currentMetrics.alertFlags` — được tính khi rollup từ ads (layer3). Gọi API `POST /meta/ad/recalculate-all` hoặc `POST /meta/campaign/recalculate` để tính lại metrics và alertFlags.\n\n")
	} else if eligibleCount == 0 {
		sb.WriteString("### 🔧 Hành động đề xuất\n\n")
		sb.WriteString("1. **Kiểm tra khớp adAccountId/ownerOrganizationId:** meta_campaigns phải có `adAccountId` và `ownerOrganizationId` trùng với ads_meta_config. Có thể campaigns thuộc ad account/org khác chưa có config.\n\n")
	} else {
		sb.WriteString("### 🔧 Nếu vẫn không tạo action\n\n")
		sb.WriteString("1. **Rule không trigger:** alertFlags có thể không match bất kỳ rule Kill/Decrease/Increase nào. Kiểm tra definitions và threshold.\n")
		sb.WriteString("2. **autoPropose = false:** ActionRuleConfig có thể tắt autoPropose cho rule tương ứng.\n")
		sb.WriteString("3. **Đã có pending:** action_pending_approval có thể đã có bản ghi pending cho campaign — hệ thống tránh duplicate.\n\n")
	}
}

func expandAdAccountIds(ids []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
		if strings.HasPrefix(id, "act_") {
			trimmed := strings.TrimPrefix(id, "act_")
			if !seen[trimmed] {
				seen[trimmed] = true
				out = append(out, trimmed)
			}
		} else {
			withAct := "act_" + id
			if !seen[withAct] {
				seen[withAct] = true
				out = append(out, withAct)
			}
		}
	}
	return out
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if n, ok := v.(float64); ok {
		return fmt.Sprintf("%.0f", n)
	}
	return fmt.Sprintf("%v", v)
}
