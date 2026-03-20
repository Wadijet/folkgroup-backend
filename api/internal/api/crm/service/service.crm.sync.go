// Package crmvc - Sync profile crm_customers từ pc_pos_customers và fb_customers.
// Hợp nhất với backfill: thông tin đến trước tạo trước, thông tin sau cập nhật thêm.
package crmvc

import (
	"context"
	"fmt"

	crmdto "meta_commerce/internal/api/crm/dto"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// SyncSourcePos và SyncSourceFb là giá trị cho tham số sources.
const (
	SyncSourcePos = "pos"
	SyncSourceFb  = "fb"
)

// SyncZaloCustomersOnly sync chỉ fb_customers nguồn Zalo (pageId bắt đầu pzl_) vào crm_customers.
// Chạy khi server khởi động để đảm bảo khách Zalo được merge.
func (s *CrmCustomerService) SyncZaloCustomersOnly(ctx context.Context) (count int, err error) {
	fbColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy fb_customers")
	}
	filter := bson.M{"pageId": bson.M{"$regex": "^pzl_"}}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "panCakeData.inserted_at", Value: 1}, {Key: "panCakeData.updated_at", Value: 1}, {Key: "_id", Value: 1}})
	cursor, err := fbColl.Find(ctx, filter, opts)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc fbmodels.FbCustomer
		if cursor.Decode(&doc) == nil {
			if e := s.MergeFromFbCustomer(ctx, &doc, 0); e == nil {
				count++
			}
		}
	}
	return count, nil
}

// SyncBatchSize kích thước batch mặc định khi sync có checkpoint.
const SyncBatchSize = 500

// SyncAllCustomers cập nhật profile crm_customers từ pc_pos_customers và fb_customers.
// Tạo mới nếu chưa có (MergeFromPosCustomer/MergeFromFbCustomer). Cập nhật thêm nếu đã có (từ order/conv).
// sources: []string{"pos","fb"} — rỗng hoặc nil = chạy tất cả.
// Xử lý từ cũ đến mới (posData/panCakeData inserted_at, updated_at asc).
// Thứ tự: pos xong hết rồi mới fb (không xen kẽ).
// progress/onProgress: nil = chạy hết 1 lần; có giá trị = chạy từng batch và lưu checkpoint.
// Progress: posSkip, fbSkip, currentSource, nextSources, percentBySource, totals.
func (s *CrmCustomerService) SyncAllCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, sources []string, progress bson.M, onProgress func(bson.M)) (posCount, fbCount int, err error) {
	if progress == nil && onProgress == nil {
		posCount, fbCount, _, _, err = s.SyncAllCustomersBatch(ctx, ownerOrgID, sources, 0, 0, 0, 0)
		return posCount, fbCount, err
	}
	posSkip, fbSkip := 0, 0
	if progress != nil {
		if p, ok := progress["posSkip"]; ok {
			posSkip = toInt(p)
		}
		if p, ok := progress["fbSkip"]; ok {
			fbSkip = toInt(p)
		}
	}
	runPos, runFb := parseSyncSources(sources)
	posTotal, fbTotal := int64(0), int64(0)
	if onProgress != nil && (runPos || runFb) {
		posTotal, fbTotal = s.countSyncSourceTotals(ctx, ownerOrgID, sources)
	}
	batchSize := SyncBatchSize

	// Phase 1: pos xong hết trước
	if runPos {
		for {
			posP, _, posMore, _, err := s.SyncAllCustomersBatch(ctx, ownerOrgID, []string{SyncSourcePos}, posSkip, batchSize, 0, 0)
			if err != nil {
				return posCount, fbCount, err
			}
			posCount += posP
			posSkip += posP
			if onProgress != nil {
				p := s.buildSyncProgress(posSkip, fbSkip, posMore, true, runPos, runFb, posTotal, fbTotal)
				onProgress(p)
			}
			if !posMore {
				break
			}
		}
	}

	// Phase 2: fb xong hết sau
	if runFb {
		for {
			_, fbP, _, fbMore, err := s.SyncAllCustomersBatch(ctx, ownerOrgID, []string{SyncSourceFb}, 0, 0, fbSkip, batchSize)
			if err != nil {
				return posCount, fbCount, err
			}
			fbCount += fbP
			fbSkip += fbP
			if onProgress != nil {
				p := s.buildSyncProgress(posSkip, fbSkip, false, fbMore, runPos, runFb, posTotal, fbTotal)
				onProgress(p)
			}
			if !fbMore {
				break
			}
		}
	}

	if onProgress != nil && (runPos || runFb) {
		onProgress(s.buildSyncProgress(posSkip, fbSkip, false, false, runPos, runFb, posTotal, fbTotal))
	}
	return posCount, fbCount, nil
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// countSyncSourceTotals đếm tổng số bản ghi mỗi nguồn (để tính % tiến độ).
func (s *CrmCustomerService) countSyncSourceTotals(ctx context.Context, ownerOrgID primitive.ObjectID, sources []string) (posTotal, fbTotal int64) {
	runPos, runFb := parseSyncSources(sources)
	if runPos {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers); ok {
			posTotal, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
		}
	}
	if runFb {
		if coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers); ok {
			fbTotal, _ = coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": ownerOrgID})
		}
	}
	return posTotal, fbTotal
}

// buildSyncProgress tạo progress với currentSource, nextSources, percentBySource.
func (s *CrmCustomerService) buildSyncProgress(posSkip, fbSkip int, posMore, fbMore, runPos, runFb bool, posTotal, fbTotal int64) bson.M {
	p := bson.M{"posSkip": posSkip, "fbSkip": fbSkip}
	percentBySource := bson.M{}
	if runPos {
		posPct := 0
		if posTotal > 0 {
			posPct = int(float64(posSkip) / float64(posTotal) * 100)
			if posPct > 100 {
				posPct = 100
			}
		}
		percentBySource["pos"] = posPct
	}
	if runFb {
		fbPct := 0
		if fbTotal > 0 {
			fbPct = int(float64(fbSkip) / float64(fbTotal) * 100)
			if fbPct > 100 {
				fbPct = 100
			}
		}
		percentBySource["fb"] = fbPct
	}
	p["percentBySource"] = percentBySource
	p["totals"] = bson.M{"pos": posTotal, "fb": fbTotal}
	if posMore {
		p["currentSource"] = "pos"
		if runFb {
			p["nextSources"] = bson.A{"fb"}
		} else {
			p["nextSources"] = bson.A{}
		}
	} else if fbMore {
		p["currentSource"] = "fb"
		p["nextSources"] = bson.A{}
	} else {
		p["currentSource"] = "done"
		p["nextSources"] = bson.A{}
	}
	return p
}

// SyncAllCustomersBatch xử lý 1 batch sync. Dùng cho checkpoint/resume.
// posSkip, posLimit, fbSkip, fbLimit: skip/limit cho từng nguồn. limit=0 = không giới hạn (xử lý hết).
// Trả về (posProcessed, fbProcessed, posHasMore, fbHasMore, error).
func (s *CrmCustomerService) SyncAllCustomersBatch(ctx context.Context, ownerOrgID primitive.ObjectID, sources []string, posSkip, posLimit, fbSkip, fbLimit int) (posCount, fbCount int, posHasMore, fbHasMore bool, err error) {
	runPos, runFb := parseSyncSources(sources)

	if runPos {
		posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
		if !ok {
			return 0, 0, false, false, fmt.Errorf("không tìm thấy pc_pos_customers")
		}
		posOpts := mongoopts.Find().SetSort(bson.D{{Key: "posData.inserted_at", Value: 1}, {Key: "posData.updated_at", Value: 1}, {Key: "_id", Value: 1}})
		if posSkip > 0 {
			posOpts.SetSkip(int64(posSkip))
		}
		if posLimit > 0 {
			posOpts.SetLimit(int64(posLimit) + 1) // +1 để biết hasMore
		}
		posCursor, err := posColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, posOpts)
		if err != nil {
			return 0, 0, false, false, err
		}
		processed := 0
		for posCursor.Next(ctx) {
			if posLimit > 0 && processed >= posLimit {
				posHasMore = true
				break
			}
			var doc pcmodels.PcPosCustomer
			if posCursor.Decode(&doc) == nil {
				if e := s.MergeFromPosCustomer(ctx, &doc, 0); e == nil {
					posCount++
				}
			}
			processed++
		}
		posCursor.Close(ctx)
	}

	if runFb {
		fbColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
		if !ok {
			return posCount, 0, posHasMore, false, fmt.Errorf("không tìm thấy fb_customers")
		}
		fbOpts := mongoopts.Find().SetSort(bson.D{{Key: "panCakeData.inserted_at", Value: 1}, {Key: "panCakeData.updated_at", Value: 1}, {Key: "_id", Value: 1}})
		if fbSkip > 0 {
			fbOpts.SetSkip(int64(fbSkip))
		}
		if fbLimit > 0 {
			fbOpts.SetLimit(int64(fbLimit) + 1)
		}
		fbCursor, err := fbColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, fbOpts)
		if err != nil {
			return posCount, 0, posHasMore, false, err
		}
		processed := 0
		for fbCursor.Next(ctx) {
			if fbLimit > 0 && processed >= fbLimit {
				fbHasMore = true
				break
			}
			var doc fbmodels.FbCustomer
			if fbCursor.Decode(&doc) == nil {
				if e := s.MergeFromFbCustomer(ctx, &doc, 0); e == nil {
					fbCount++
				}
			}
			processed++
		}
		fbCursor.Close(ctx)
	}

	return posCount, fbCount, posHasMore, fbHasMore, nil
}

// parseSyncSources parse sources thành runPos, runFb. Rỗng/nil = cả hai.
func parseSyncSources(sources []string) (runPos, runFb bool) {
	if len(sources) == 0 {
		return true, true
	}
	for _, v := range sources {
		switch v {
		case SyncSourcePos:
			runPos = true
		case SyncSourceFb:
			runFb = true
		}
	}
	return runPos, runFb
}

// progressInt lấy int từ map (BSON có thể trả về int32, int64, float64).
func progressInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// RebuildCrm chạy sync rồi backfill. Thứ tự: sync trước (profile từ POS/FB), backfill sau (activity từ orders/conversations/notes).
// sources/types: nil = chạy tất cả; [] (rỗng) = bỏ qua phần đó; [a,b] = chỉ chạy a,b.
// progress: tiến độ để resume (nil = bắt đầu mới). onProgress: callback sau mỗi batch (nil = không dùng checkpoint).
func (s *CrmCustomerService) RebuildCrm(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, sources, types []string, progress bson.M, onProgress func(bson.M)) (*crmdto.CrmRebuildResult, error) {
	result := &crmdto.CrmRebuildResult{}
	batchSize := SyncBatchSize

	posSkip, fbSkip := 0, 0
	if progress != nil {
		if p, ok := progress["sync"].(map[string]interface{}); ok {
			posSkip = progressInt(p, "posSkip")
			fbSkip = progressInt(p, "fbSkip")
		}
	}

	runPos, runFb := parseSyncSources(sources)
	posTotal, fbTotal := int64(0), int64(0)
	if onProgress != nil && (sources == nil || len(sources) > 0) && (runPos || runFb) {
		posTotal, fbTotal = s.countSyncSourceTotals(ctx, ownerOrgID, sources)
	}

	// Sync trước: pos xong hết rồi mới fb
	var syncProgress bson.M
	if sources == nil || len(sources) > 0 {
		if runPos {
			for {
				posCount, _, posMore, _, err := s.SyncAllCustomersBatch(ctx, ownerOrgID, []string{SyncSourcePos}, posSkip, batchSize, 0, 0)
				if err != nil {
					return nil, err
				}
				result.Sync.PosProcessed += posCount
				posSkip += posCount
				syncProgress = s.buildSyncProgress(posSkip, fbSkip, posMore, false, runPos, runFb, posTotal, fbTotal)
				if onProgress != nil {
					onProgress(bson.M{"phase": "sync", "currentSource": syncProgress["currentSource"], "nextSources": syncProgress["nextSources"], "percentBySource": syncProgress["percentBySource"], "sync": bson.M{"posSkip": posSkip, "fbSkip": fbSkip, "totals": syncProgress["totals"]}})
				}
				if !posMore {
					break
				}
			}
		}
		if runFb {
			for {
				_, fbCount, _, fbMore, err := s.SyncAllCustomersBatch(ctx, ownerOrgID, []string{SyncSourceFb}, 0, 0, fbSkip, batchSize)
				if err != nil {
					return nil, err
				}
				result.Sync.FbProcessed += fbCount
				fbSkip += fbCount
				syncProgress = s.buildSyncProgress(posSkip, fbSkip, false, fbMore, runPos, runFb, posTotal, fbTotal)
				if onProgress != nil {
					onProgress(bson.M{"phase": "sync", "currentSource": syncProgress["currentSource"], "nextSources": syncProgress["nextSources"], "percentBySource": syncProgress["percentBySource"], "sync": bson.M{"posSkip": posSkip, "fbSkip": fbSkip, "totals": syncProgress["totals"]}})
				}
				if !fbMore {
					break
				}
			}
		}
	}
	if syncProgress == nil {
		syncProgress = bson.M{"posSkip": posSkip, "fbSkip": fbSkip}
	}

	// Backfill sau: đẩy activity từ orders, conversations, notes vào crm_activity_history (có checkpoint)
	if types == nil || len(types) > 0 {
		var onBackfillProgress func(bson.M)
		if onProgress != nil {
			onBackfillProgress = func(bf bson.M) {
				combined := bson.M{"phase": "backfill", "sync": syncProgress, "backfill": bf}
				if cs, ok := bf["currentSource"]; ok {
					combined["currentSource"] = cs
				}
				if ns, ok := bf["nextSources"]; ok {
					combined["nextSources"] = ns
				}
				if pbs, ok := bf["percentBySource"]; ok {
					combined["percentBySource"] = pbs
				}
				onProgress(combined)
			}
		}
		backfillResult, err := s.BackfillActivity(ctx, ownerOrgID, limit, types, progress, onBackfillProgress)
		if err != nil {
			return nil, err
		}
		result.Backfill = *backfillResult
	}

	return result, nil
}
