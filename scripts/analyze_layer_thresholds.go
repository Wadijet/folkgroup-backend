// Script phân tích dữ liệu crm_customers để tạo căn cứ điều chỉnh threshold customer (Lớp 2, Lớp 3).
// Aggregate từ currentMetrics.raw, tính percentile, phân bucket, xuất báo cáo markdown.
//
// Chạy: cd api && go run ../scripts/analyze_layer_thresholds.go
// Output: scripts/reports/BAO_CAO_THRESHOLD_LAYER_YYYYMMDD.md
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"meta_commerce/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	colCrmCustomers = "crm_customers"
	msPerDay        = 24 * 60 * 60 * 1000
)

func main() {
	fmt.Println("=== Phân Tích Threshold Layer — Căn Cứ Điều Chỉnh Customer ===\n")

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Không thể ping MongoDB: %v", err)
	}

	db := client.Database(cfg.MongoDB_DBName_Auth)
	coll := db.Collection(colCrmCustomers)

	now := time.Now()
	reportDate := now.Format("20060102")
	var sb strings.Builder

	// Header
	sb.WriteString("# BÁO CÁO PHÂN TÍCH THRESHOLD LAYER — CĂN CỨ ĐIỀU CHỈNH\n\n")
	sb.WriteString(fmt.Sprintf("**Ngày tạo:** %s  \n**Database:** %s  \n**Nguồn:** crm_customers.currentMetrics\n\n", now.Format("2006-01-02 15:04"), cfg.MongoDB_DBName_Auth))
	sb.WriteString("---\n\n")

	// Project để lấy currentMetrics + classification (layer1 có journeyStage, layer2 có valueTier, lifecycleStage)
	projectStage := bson.M{
		"$project": bson.M{
			"raw":           "$currentMetrics.raw",
			"layer1":        "$currentMetrics.layer1",
			"layer2":        "$currentMetrics.layer2",
			"journeyStage":  bson.M{"$ifNull": bson.A{"$currentMetrics.layer1.journeyStage", "$journeyStage"}},
			"valueTier":     bson.M{"$ifNull": bson.A{"$currentMetrics.layer2.valueTier", "$valueTier"}},
			"lifecycleStage": bson.M{"$ifNull": bson.A{"$currentMetrics.layer2.lifecycleStage", "$lifecycleStage"}},
		},
	}
	matchStage := bson.M{"$match": bson.M{"currentMetrics.raw": bson.M{"$exists": true}}}

	cur, err := coll.Aggregate(ctx, []bson.M{matchStage, projectStage})
	if err != nil {
		log.Fatalf("Aggregate: %v", err)
	}
	defer cur.Close(ctx)

	var (
		totalSpentVals     []float64
		orderCountVals     []int
		daysSinceVals      []int64
		avgOrderValueVals  []float64
		totalMessagesVals  []int
		ownedSkuCountVals  []int
		rev30Rev90Vals     []float64
		engagedTotalMsg    []int
		firstAOVVals       []float64
		repeatAvgDaysVals  []float64
		valueTierCount     = map[string]int{}
		lifecycleCount     = map[string]int{}
		journeyCount       = map[string]int{}
		totalWithMetrics   int
		totalWithOrders    int
		totalEngaged       int
	)

	for cur.Next(ctx) {
		var doc struct {
			Raw      map[string]interface{} `bson:"raw"`
			Layer2   map[string]interface{} `bson:"layer2"`
			Journey  string                 `bson:"journeyStage"`
			ValueTier string                `bson:"valueTier"`
			Lifecycle string                `bson:"lifecycleStage"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		totalWithMetrics++
		raw := doc.Raw
		if raw == nil {
			continue
		}

		// Classification counts
		jt := doc.Journey
		if jt == "" {
			jt = "_unspecified"
		}
		journeyCount[jt]++
		vt := doc.ValueTier
		if vt == "" {
			vt = "_unspecified"
		}
		valueTierCount[vt]++
		lc := doc.Lifecycle
		if lc == "" {
			lc = "_unspecified"
		}
		lifecycleCount[lc]++

		// Numeric metrics
		ts := toFloat(raw["totalSpent"])
		oc := toInt(raw["orderCount"])
		lastOrder := toInt64(raw["lastOrderAt"])
		aov := toFloat(raw["avgOrderValue"])
		rev30 := toFloat(raw["revenueLast30d"])
		rev90 := toFloat(raw["revenueLast90d"])
		totalMsg := toInt(raw["totalMessages"])
		skuCount := toInt(raw["ownedSkuCount"])
		secondLast := toInt64(raw["secondLastOrderAt"])

		if oc > 0 {
			totalWithOrders++
		}

		// totalSpent (chỉ khách có đơn)
		if oc > 0 && ts >= 0 {
			totalSpentVals = append(totalSpentVals, ts)
		}

		orderCountVals = append(orderCountVals, oc)

		// daysSinceLastOrder
		if lastOrder > 0 {
			nowMs := time.Now().UnixMilli()
			if lastOrder < 1e12 {
				lastOrder *= 1000
			}
			days := (nowMs - lastOrder) / msPerDay
			if days >= 0 {
				daysSinceVals = append(daysSinceVals, days)
			}
		}

		// avgOrderValue (First: oc=1)
		if oc == 1 && aov > 0 {
			firstAOVVals = append(firstAOVVals, aov)
		}
		if oc > 0 && aov > 0 {
			avgOrderValueVals = append(avgOrderValueVals, aov)
		}

		// totalMessages
		if totalMsg >= 0 {
			totalMessagesVals = append(totalMessagesVals, totalMsg)
		}
		if jt == "engaged" && oc == 0 {
			totalEngaged++
			engagedTotalMsg = append(engagedTotalMsg, totalMsg)
		}

		// ownedSkuCount (Repeat/VIP)
		if oc >= 2 && skuCount >= 0 {
			ownedSkuCountVals = append(ownedSkuCountVals, skuCount)
		}

		// rev30/rev90 (momentum)
		if rev90 > 0 && rev30 >= 0 {
			rev30Rev90Vals = append(rev30Rev90Vals, rev30/rev90)
		}

		// avgDaysBetweenOrders (Repeat)
		if oc >= 2 && lastOrder > 0 && secondLast > 0 && secondLast < lastOrder {
			if lastOrder < 1e12 {
				lastOrder *= 1000
			}
			if secondLast < 1e12 {
				secondLast *= 1000
			}
			avgDays := float64(lastOrder-secondLast) / float64(msPerDay)
			if avgDays > 0 {
				repeatAvgDaysVals = append(repeatAvgDaysVals, avgDays)
			}
		}
	}

	if err := cur.Err(); err != nil {
		log.Fatalf("Cursor: %v", err)
	}

	// Section 1: Tổng quan
	writeSection1(&sb, totalWithMetrics, totalWithOrders, totalEngaged)

	// Section 2: totalSpent
	writeSection2(&sb, totalSpentVals)

	// Section 3: orderCount
	writeSection3(&sb, orderCountVals)

	// Section 4: daysSinceLastOrder
	writeSection4(&sb, daysSinceVals)

	// Section 4b: avgDaysBetweenOrders
	writeSection4b(&sb, repeatAvgDaysVals)

	// Section 5: avgOrderValue (First)
	writeSection5(&sb, firstAOVVals)

	// Section 5b: totalMessages (Engaged)
	writeSection5b(&sb, engagedTotalMsg, totalMessagesVals)

	// Section 5c: ownedSkuCount
	writeSection5c(&sb, ownedSkuCountVals)

	// Section 6: rev30/rev90, spend momentum
	writeSection6(&sb, rev30Rev90Vals)

	// Section 7: Phân bố classification
	writeSection7(&sb, valueTierCount, lifecycleCount, journeyCount)

	// Ghi file
	reportDir := filepath.Join("..", "scripts", "reports")
	if wd, _ := os.Getwd(); !strings.Contains(wd, "api") {
		reportDir = filepath.Join("scripts", "reports")
	}
	_ = os.MkdirAll(reportDir, 0755)
	outPath := filepath.Join(reportDir, fmt.Sprintf("BAO_CAO_THRESHOLD_LAYER_%s.md", reportDate))
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("Ghi file: %v", err)
	}
	fmt.Printf("✅ Đã tạo báo cáo: %s\n", outPath)
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func toInt64(v interface{}) int64 {
	if v == nil {
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

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(idx-float64(lo))
}

func percentileInt(sorted []int, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return float64(sorted[lo])
	}
	return float64(sorted[lo]) + (float64(sorted[hi])-float64(sorted[lo]))*(idx-float64(lo))
}

func writeSection1(sb *strings.Builder, total, withOrders, engaged int) {
	sb.WriteString("## 1. TỔNG QUAN\n\n")
	sb.WriteString("| Chỉ số | Giá trị |\n|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| Tổng crm_customers (có currentMetrics.raw) | %d |\n", total))
	sb.WriteString(fmt.Sprintf("| Có orderCount > 0 | %d |\n", withOrders))
	sb.WriteString(fmt.Sprintf("| Có orderCount = 0 | %d |\n", total-withOrders))
	sb.WriteString(fmt.Sprintf("| Engaged (journeyStage=engaged, orderCount=0) | %d |\n", engaged))
	sb.WriteString("\n")
}

func writeSection2(sb *strings.Builder, vals []float64) {
	sb.WriteString("## 2. PHÂN PHỐI TOTALSPENT (VNĐ) — Lớp 2 valueTier\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu (chỉ khách có đơn).*\n\n")
		return
	}
	sort.Float64s(vals)
	sb.WriteString("| Chỉ số | Giá trị |\n|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| Min | %.0f |\n", vals[0]))
	sb.WriteString(fmt.Sprintf("| Max | %.0f |\n", vals[len(vals)-1]))
	sb.WriteString(fmt.Sprintf("| P25 | %.0f |\n", percentile(vals, 25)))
	sb.WriteString(fmt.Sprintf("| P50 | %.0f |\n", percentile(vals, 50)))
	sb.WriteString(fmt.Sprintf("| P75 | %.0f |\n", percentile(vals, 75)))
	sb.WriteString(fmt.Sprintf("| P90 | %.0f |\n", percentile(vals, 90)))
	sb.WriteString(fmt.Sprintf("| P95 | %.0f |\n", percentile(vals, 95)))
	// Bucket
	buckets := []struct{ label string; min, max float64 }{
		{"0–1M", 0, 1e6},
		{"1M–5M", 1e6, 5e6},
		{"5M–20M", 5e6, 20e6},
		{"20M–50M", 20e6, 50e6},
		{"50M+", 50e6, 1e18},
	}
	sb.WriteString("\n| Khoảng | Số khách |\n|--------|----------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if v >= b.min && v < b.max {
				n++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", b.label, n))
	}
	sb.WriteString("\n")
}

func writeSection3(sb *strings.Builder, vals []int) {
	sb.WriteString("## 3. PHÂN PHỐI ORDERCOUNT — Lớp 2 loyalty, Lớp 3 journey/VIP\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu.*\n\n")
		return
	}
	sort.Ints(vals)
	buckets := []struct{ label string; min, max int }{
		{"0", 0, 0},
		{"1", 1, 1},
		{"2–7", 2, 7},
		{"8+", 8, 999},
	}
	sb.WriteString("| orderCount | Số khách | Ghi chú |\n|------------|----------|--------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if v >= b.min && v <= b.max {
				n++
			}
		}
		note := ""
		switch b.label {
		case "0":
			note = "visitor/engaged"
		case "1":
			note = "first"
		case "2–7":
			note = "repeat"
		case "8+":
			note = "VIP"
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", b.label, n, note))
	}
	sb.WriteString("\n")
}

func writeSection4(sb *strings.Builder, vals []int64) {
	sb.WriteString("## 4. PHÂN PHỐI DAYSSINCELASTORDER — Lớp 2 lifecycleStage\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu (chỉ khách có đơn).*\n\n")
		return
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	floats := make([]float64, len(vals))
	for i, v := range vals {
		floats[i] = float64(v)
	}
	sb.WriteString("| Chỉ số | Giá trị (ngày) |\n|--------|----------------|\n")
	sb.WriteString(fmt.Sprintf("| Min | %.0f |\n", floats[0]))
	sb.WriteString(fmt.Sprintf("| Max | %.0f |\n", floats[len(floats)-1]))
	sb.WriteString(fmt.Sprintf("| P50 | %.0f |\n", percentile(floats, 50)))
	sb.WriteString(fmt.Sprintf("| P75 | %.0f |\n", percentile(floats, 75)))
	sb.WriteString(fmt.Sprintf("| P90 | %.0f |\n", percentile(floats, 90)))
	buckets := []struct{ label string; min, max int64 }{
		{"0–30 (active)", 0, 30},
		{"31–90 (cooling)", 31, 90},
		{"91–180 (inactive)", 91, 180},
		{"181+ (dead)", 181, 99999},
	}
	sb.WriteString("\n| Khoảng | Số khách |\n|--------|----------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if v >= b.min && v <= b.max {
				n++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", b.label, n))
	}
	sb.WriteString("\n")
}

func writeSection4b(sb *strings.Builder, vals []float64) {
	sb.WriteString("## 4b. PHÂN PHỐI AVGDAYSBETWEENORDERS — Lớp 3 Repeat repeatFrequency\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu (chỉ khách repeat có secondLastOrderAt).*\n\n")
		return
	}
	sort.Float64s(vals)
	sb.WriteString("| Chỉ số | Giá trị (ngày) |\n|--------|----------------|\n")
	sb.WriteString(fmt.Sprintf("| Min | %.1f |\n", vals[0]))
	sb.WriteString(fmt.Sprintf("| Max | %.1f |\n", vals[len(vals)-1]))
	sb.WriteString(fmt.Sprintf("| P25 | %.1f |\n", percentile(vals, 25)))
	sb.WriteString(fmt.Sprintf("| P50 | %.1f |\n", percentile(vals, 50)))
	sb.WriteString(fmt.Sprintf("| P75 | %.1f |\n", percentile(vals, 75)))
	sb.WriteString("\n")
}

func writeSection5(sb *strings.Builder, vals []float64) {
	sb.WriteString("## 5. PHÂN PHỐI AVGORDERVALUE (First, orderCount=1) — Lớp 3 purchaseQuality\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu.*\n\n")
		return
	}
	sort.Float64s(vals)
	sb.WriteString("| Chỉ số | Giá trị (VNĐ) |\n|--------|---------------|\n")
	sb.WriteString(fmt.Sprintf("| Min | %.0f |\n", vals[0]))
	sb.WriteString(fmt.Sprintf("| Max | %.0f |\n", vals[len(vals)-1]))
	sb.WriteString(fmt.Sprintf("| P25 | %.0f |\n", percentile(vals, 25)))
	sb.WriteString(fmt.Sprintf("| P50 | %.0f |\n", percentile(vals, 50)))
	sb.WriteString(fmt.Sprintf("| P75 | %.0f |\n", percentile(vals, 75)))
	buckets := []struct{ label string; min, max float64 }{
		{"< 150k (entry)", 0, 150000},
		{"150k–500k (medium)", 150000, 500000},
		{"≥ 500k (high_aov)", 500000, 1e18},
	}
	sb.WriteString("\n| Khoảng | Số khách |\n|--------|----------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if v >= b.min && v < b.max {
				n++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", b.label, n))
	}
	sb.WriteString("\n")
}

func writeSection5b(sb *strings.Builder, engagedMsg, allMsg []int) {
	sb.WriteString("## 5b. PHÂN PHỐI TOTALMESSAGES — Lớp 3 Engaged engagementDepth\n\n")
	sb.WriteString("**Chỉ khách engaged (journeyStage=engaged, orderCount=0):**\n\n")
	if len(engagedMsg) == 0 {
		sb.WriteString("*Không có dữ liệu engaged.*\n\n")
	} else {
		sort.Ints(engagedMsg)
		sb.WriteString("| Chỉ số | Giá trị (số tin) |\n|--------|------------------|\n")
		sb.WriteString(fmt.Sprintf("| Min | %d |\n", engagedMsg[0]))
		sb.WriteString(fmt.Sprintf("| Max | %d |\n", engagedMsg[len(engagedMsg)-1]))
		sb.WriteString(fmt.Sprintf("| P25 | %.0f |\n", percentileInt(engagedMsg, 25)))
		sb.WriteString(fmt.Sprintf("| P50 | %.0f |\n", percentileInt(engagedMsg, 50)))
		sb.WriteString(fmt.Sprintf("| P75 | %.0f |\n", percentileInt(engagedMsg, 75)))
		buckets := []struct{ label string; min, max int }{
			{"0", 0, 0},
			{"1–3 (light)", 1, 3},
			{"4–10 (medium)", 4, 10},
			{"11+ (deep)", 11, 99999},
		}
		sb.WriteString("\n| Khoảng | Số khách engaged |\n|--------|-------------------|\n")
		for _, b := range buckets {
			n := 0
			for _, v := range engagedMsg {
				if v >= b.min && v <= b.max {
					n++
				}
			}
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", b.label, n))
		}
	}
	sb.WriteString("\n**Toàn bộ (có totalMessages):** ")
	sb.WriteString(fmt.Sprintf("%d khách\n\n", len(allMsg)))
}

func writeSection5c(sb *strings.Builder, vals []int) {
	sb.WriteString("## 5c. PHÂN PHỐI OWNEDSKUCOUNT — Lớp 3 Repeat/VIP productExpansion/productDiversity\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu (chỉ khách repeat/VIP).*\n\n")
		return
	}
	sort.Ints(vals)
	buckets := []struct{ label string; min, max int }{
		{"0–2 (single)", 0, 2},
		{"3–7 (multi)", 3, 7},
		{"8+ (full_portfolio)", 8, 99999},
	}
	sb.WriteString("| Khoảng | Số khách |\n|--------|----------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if v >= b.min && v <= b.max {
				n++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", b.label, n))
	}
	sb.WriteString("\n")
}

func writeSection6(sb *strings.Builder, vals []float64) {
	sb.WriteString("## 6. PHÂN PHỐI REV30/REV90 — Lớp 2 momentumStage\n\n")
	if len(vals) == 0 {
		sb.WriteString("*Không có dữ liệu (chỉ khách có revenueLast90d > 0).*\n\n")
		return
	}
	sort.Float64s(vals)
	buckets := []struct{ label string; min, max float64 }{
		{"> 0.5 (rising)", 0.5, 10},
		{"0.2–0.5 (stable)", 0.2, 0.5},
		{"< 0.2 (declining/lost)", 0, 0.2},
	}
	sb.WriteString("| Ratio | Số khách | momentumStage |\n|-------|----------|---------------|\n")
	for _, b := range buckets {
		n := 0
		for _, v := range vals {
			if b.label == "> 0.5 (rising)" && v > 0.5 {
				n++
			} else if b.label == "0.2–0.5 (stable)" && v >= 0.2 && v <= 0.5 {
				n++
			} else if b.label == "< 0.2 (declining/lost)" && v < 0.2 {
				n++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | |\n", b.label, n))
	}
	sb.WriteString("\n")
}

func writeSection7(sb *strings.Builder, valueTier, lifecycle, journey map[string]int) {
	sb.WriteString("## 7. PHÂN BỐ HIỆN TẠI THEO CLASSIFICATION\n\n")
	total := 0
	for _, n := range valueTier {
		total += n
	}
	if total == 0 {
		sb.WriteString("*Không có dữ liệu.*\n\n")
		return
	}
	order := []string{"new", "low", "medium", "high", "top", "_unspecified"}
	sb.WriteString("| valueTier | Số khách | % |\n|-----------|----------|---|\n")
	for _, k := range order {
		if n, ok := valueTier[k]; ok && n > 0 {
			sb.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", k, n, float64(n)/float64(total)*100))
		}
	}
	sb.WriteString("\n| lifecycleStage | Số khách | % |\n|----------------|----------|---|\n")
	lcOrder := []string{"active", "cooling", "inactive", "dead", "_unspecified"}
	for _, k := range lcOrder {
		if n, ok := lifecycle[k]; ok && n > 0 {
			sb.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", k, n, float64(n)/float64(total)*100))
		}
	}
	sb.WriteString("\n| journeyStage | Số khách | % |\n|--------------|----------|---|\n")
	jOrder := []string{"visitor", "engaged", "first", "repeat", "promoter", "blocked_spam", "_unspecified"}
	for _, k := range jOrder {
		if n, ok := journey[k]; ok && n > 0 {
			sb.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", k, n, float64(n)/float64(total)*100))
		}
	}
	sb.WriteString("\n")
}
