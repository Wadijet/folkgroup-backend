// Package reportsvc — Touch báo cáo trong bộ nhớ process (datachanged → ff:rt:*; worker flush → MarkDirty).
// Không dùng Redis — dữ liệu chỉ trên một instance API/worker; scale ngang cần cơ chế khác.
package reportsvc

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	redisTouchPrefixOrder    = "ff:rt:o|"
	redisTouchPrefixCustomer = "ff:rt:c|"
	redisTouchPrefixAds      = "ff:rt:a|"
)

// Nhóm flush touch trong RAM → MarkDirty (một worker gọi từng domain theo lịch riêng).
const (
	ReportRedisFlushDomainAds      = "ads"
	ReportRedisFlushDomainOrder    = "order"
	ReportRedisFlushDomainCustomer = "customer"
)

type reportTouchEntry struct {
	val       string
	expiresAt time.Time // zero = không hết hạn
}

var (
	reportTouchMu   sync.Mutex
	reportTouchKeys map[string]*reportTouchEntry
)

func reportTouchInitMap() {
	if reportTouchKeys == nil {
		reportTouchKeys = make(map[string]*reportTouchEntry)
	}
}

func reportTouchSet(ctx context.Context, key, val string) {
	_ = ctx
	ttl := reportRedisTouchTTL()
	reportTouchMu.Lock()
	defer reportTouchMu.Unlock()
	reportTouchInitMap()
	e := &reportTouchEntry{val: val}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	reportTouchKeys[key] = e
}

func reportTouchDelete(key string) {
	reportTouchMu.Lock()
	defer reportTouchMu.Unlock()
	delete(reportTouchKeys, key)
}

func reportTouchGet(key string) (string, bool) {
	reportTouchMu.Lock()
	defer reportTouchMu.Unlock()
	e, ok := reportTouchKeys[key]
	if !ok {
		return "", false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(reportTouchKeys, key)
		return "", false
	}
	return e.val, true
}

// keysMatching snapshot các key còn hiệu lực; match dạng prefix* (vd. ff:rt:a|*).
func reportTouchKeysMatching(match string) []string {
	prefix := strings.TrimSuffix(match, "*")
	reportTouchMu.Lock()
	defer reportTouchMu.Unlock()
	now := time.Now()
	var out []string
	for k, e := range reportTouchKeys {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(reportTouchKeys, k)
			continue
		}
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out
}

// GetReportRedisTouchFlushIntervals trả về chu kỳ flush theo từng loại (từ config; tối thiểu 5s mỗi nhánh).
func GetReportRedisTouchFlushIntervals() (ads, order, customer time.Duration) {
	cfg := global.MongoDB_ServerConfig
	defAds, defOrd, defCust := 30*time.Second, 30*time.Second, 30*time.Second
	if cfg == nil {
		return defAds, defOrd, defCust
	}
	return durFromSecMin(cfg.ReportRedisTouchFlushIntervalAdsSec, defAds, 5),
		durFromSecMin(cfg.ReportRedisTouchFlushIntervalOrderSec, defOrd, 5),
		durFromSecMin(cfg.ReportRedisTouchFlushIntervalCustomerSec, defCust, 5)
}

// GetReportRedisTouchPollTick bước ngủ giữa các vòng kiểm tra trong worker (mặc định 3s).
func GetReportRedisTouchPollTick() time.Duration {
	cfg := global.MongoDB_ServerConfig
	sec := 3
	if cfg != nil && cfg.ReportRedisTouchPollTickSec > 0 {
		sec = cfg.ReportRedisTouchPollTickSec
	}
	if sec < 2 {
		sec = 2
	}
	return time.Duration(sec) * time.Second
}

func durFromSecMin(sec int, def time.Duration, minSec int) time.Duration {
	if sec <= 0 {
		return def
	}
	if sec < minSec {
		sec = minSec
	}
	return time.Duration(sec) * time.Second
}

func redisScanMatchForDomain(domain string) (string, error) {
	switch domain {
	case ReportRedisFlushDomainAds:
		return redisTouchPrefixAds + "*", nil
	case ReportRedisFlushDomainOrder:
		return redisTouchPrefixOrder + "*", nil
	case ReportRedisFlushDomainCustomer:
		return redisTouchPrefixCustomer + "*", nil
	default:
		return "", fmt.Errorf("domain flush touch không hợp lệ: %q (ads|order|customer)", domain)
	}
}

func reportRedisTouchTTL() time.Duration {
	cfg := global.MongoDB_ServerConfig
	sec := 7200
	if cfg != nil && cfg.ReportRedisTouchTTLSec > 0 {
		sec = cfg.ReportRedisTouchTTLSec
	}
	return time.Duration(sec) * time.Second
}

// RecordReportTouchFromDataChange ghi key trong RAM; logic lọc giữ nguyên.
func RecordReportTouchFromDataChange(ctx context.Context, e events.DataChangeEvent) {
	if e.Document == nil {
		return
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}
	orgHex := ownerOrgID.Hex()

	switch e.CollectionName {
	case global.MongoDB_ColNames.PcPosOrders, global.MongoDB_ColNames.ManualPosOrders, global.MongoDB_ColNames.OrderCanonical:
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
		ts := events.GetInt64Field(e.Document, "PosCreatedAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "InsertedAt")
		}
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			return
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		reportSvc, err := NewReportService()
		if err != nil {
			return
		}
		orderReportKeys := GetActiveOrderReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, orderReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		reportTouchSet(ctx, redisTouchPrefixOrder+orgHex, strconv.FormatInt(ts, 10))

	case global.MongoDB_ColNames.PcPosCustomers:
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
		ts := events.GetInt64Field(e.Document, "UpdatedAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "LastOrderAt")
		}
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		reportSvc, err := NewReportService()
		if err != nil {
			return
		}
		customerReportKeys := GetActiveCustomerReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		reportTouchSet(ctx, redisTouchPrefixCustomer+orgHex, strconv.FormatInt(ts, 10))

	case global.MongoDB_ColNames.CustomerActivityHistory:
		if e.Operation == events.OpUpdate && e.PreviousDocument != nil {
			tsNew := events.GetPeriodTimestamp(e.Document, e.CollectionName)
			tsPrev := events.GetPeriodTimestamp(e.PreviousDocument, e.CollectionName)
			if tsNew == tsPrev && tsNew != 0 {
				return
			}
		}
		ts := events.GetInt64Field(e.Document, "ActivityAt")
		if ts == 0 {
			ts = events.GetInt64Field(e.Document, "CreatedAt")
		}
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if ts > 1e12 {
			ts = ts / 1000
		}
		reportSvc, err := NewReportService()
		if err != nil {
			return
		}
		customerReportKeys := GetActiveCustomerReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			return
		}
		reportTouchSet(ctx, redisTouchPrefixCustomer+orgHex, strconv.FormatInt(ts, 10))

	case global.MongoDB_ColNames.MetaAdInsights:
		dateStart := events.GetStringField(e.Document, "DateStart")
		adAccountId := events.GetStringField(e.Document, "AdAccountId")
		if dateStart == "" || adAccountId == "" || IsAdsReportKeyDisabled("ads_daily") {
			return
		}
		esc := url.QueryEscape(adAccountId)
		key := redisTouchPrefixAds + orgHex + "|" + esc + "|" + dateStart
		reportTouchSet(ctx, key, "1")
	default:
		return
	}
}

// FlushReportTouchesForDomain quét RAM theo nhóm (ads | order | customer) → MarkDirty rồi xóa key.
func FlushReportTouchesForDomain(ctx context.Context, domain string) (int, error) {
	match, err := redisScanMatchForDomain(domain)
	if err != nil {
		return 0, err
	}
	return flushReportTouchesScanMatch(ctx, match)
}

// FlushReportTouchesFromRedis quét toàn bộ ff:rt:* (tên hàm giữ tương thích; không còn Redis).
func FlushReportTouchesFromRedis(ctx context.Context) (int, error) {
	return flushReportTouchesScanMatch(ctx, "ff:rt:*")
}

func flushReportTouchesScanMatch(ctx context.Context, match string) (int, error) {
	reportSvc, err := NewReportService()
	if err != nil {
		return 0, err
	}
	keys := reportTouchKeysMatching(match)
	n := 0
	for _, key := range keys {
		if flushOneReportTouchKey(ctx, reportSvc, key) {
			n++
		}
	}
	return n, nil
}

func flushOneReportTouchKey(ctx context.Context, reportSvc *ReportService, key string) bool {
	switch {
	case strings.HasPrefix(key, redisTouchPrefixOrder):
		orgHex := strings.TrimPrefix(key, redisTouchPrefixOrder)
		oid, err := primitive.ObjectIDFromHex(orgHex)
		if err != nil {
			reportTouchDelete(key)
			return false
		}
		val, ok := reportTouchGet(key)
		if !ok {
			return false
		}
		ts := time.Now().Unix()
		if v := strings.TrimSpace(val); v != "" {
			if x, e := strconv.ParseInt(v, 10, 64); e == nil && x > 0 {
				ts = x
			}
		}
		orderReportKeys := GetActiveOrderReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, orderReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			reportTouchDelete(key)
			return false
		}
		markDirtyForPeriods(ctx, reportSvc, periodKeys, oid)
		reportTouchDelete(key)
		return true

	case strings.HasPrefix(key, redisTouchPrefixCustomer):
		orgHex := strings.TrimPrefix(key, redisTouchPrefixCustomer)
		oid, err := primitive.ObjectIDFromHex(orgHex)
		if err != nil {
			reportTouchDelete(key)
			return false
		}
		val, ok := reportTouchGet(key)
		if !ok {
			return false
		}
		ts := time.Now().Unix()
		if v := strings.TrimSpace(val); v != "" {
			if x, e := strconv.ParseInt(v, 10, 64); e == nil && x > 0 {
				ts = x
			}
		}
		customerReportKeys := GetActiveCustomerReportKeys()
		periodKeys, err := reportSvc.GetDirtyPeriodKeysForReportKeys(ctx, customerReportKeys, ts)
		if err != nil || len(periodKeys) == 0 {
			reportTouchDelete(key)
			return false
		}
		markDirtyForPeriods(ctx, reportSvc, periodKeys, oid)
		reportTouchDelete(key)
		return true

	case strings.HasPrefix(key, redisTouchPrefixAds):
		rest := strings.TrimPrefix(key, redisTouchPrefixAds)
		parts := strings.SplitN(rest, "|", 3)
		if len(parts) != 3 {
			reportTouchDelete(key)
			return false
		}
		orgHex, escAcc, dateStart := parts[0], parts[1], parts[2]
		oid, err := primitive.ObjectIDFromHex(orgHex)
		if err != nil {
			reportTouchDelete(key)
			return false
		}
		adAccountId, err := url.QueryUnescape(escAcc)
		if err != nil || adAccountId == "" {
			reportTouchDelete(key)
			return false
		}
		_ = reportSvc.MarkDirtyAdsDaily(ctx, dateStart, oid, adAccountId)
		reportTouchDelete(key)
		return true
	default:
		return false
	}
}
