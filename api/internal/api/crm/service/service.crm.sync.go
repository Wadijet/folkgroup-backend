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

// SyncAllCustomers cập nhật profile crm_customers từ pc_pos_customers và fb_customers.
// Tạo mới nếu chưa có (MergeFromPosCustomer/MergeFromFbCustomer). Cập nhật thêm nếu đã có (từ order/conv).
// sources: []string{"pos","fb"} — rỗng hoặc nil = chạy tất cả.
// Xử lý từ cũ đến mới (posData/panCakeData inserted_at, updated_at asc).
func (s *CrmCustomerService) SyncAllCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, sources []string) (posCount, fbCount int, err error) {
	runPos, runFb := parseSyncSources(sources)

	// Sync POS trước (ưu tiên vì có metrics). Xử lý từ cũ đến mới theo thời gian trong posData.
	if runPos {
		posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
		if !ok {
			return 0, 0, fmt.Errorf("không tìm thấy pc_pos_customers")
		}
		posOpts := mongoopts.Find().SetSort(bson.D{{Key: "posData.inserted_at", Value: 1}, {Key: "posData.updated_at", Value: 1}})
		posCursor, err := posColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, posOpts)
		if err != nil {
			return 0, 0, err
		}
		defer posCursor.Close(ctx)
		for posCursor.Next(ctx) {
			var doc pcmodels.PcPosCustomer
			if posCursor.Decode(&doc) == nil {
				if e := s.MergeFromPosCustomer(ctx, &doc, 0); e == nil {
					posCount++
				}
			}
		}
	}

	// Sync FB. Xử lý từ cũ đến mới theo thời gian trong panCakeData.
	if runFb {
		fbColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
		if !ok {
			return posCount, 0, fmt.Errorf("không tìm thấy fb_customers")
		}
		fbOpts := mongoopts.Find().SetSort(bson.D{{Key: "panCakeData.inserted_at", Value: 1}, {Key: "panCakeData.updated_at", Value: 1}})
		fbCursor, err := fbColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, fbOpts)
		if err != nil {
			return posCount, 0, err
		}
		defer fbCursor.Close(ctx)
		for fbCursor.Next(ctx) {
			var doc fbmodels.FbCustomer
			if fbCursor.Decode(&doc) == nil {
				if e := s.MergeFromFbCustomer(ctx, &doc, 0); e == nil {
					fbCount++
				}
			}
		}
	}

	return posCount, fbCount, nil
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

// RebuildCrm chạy backfill rồi sync. Thứ tự: backfill trước, sync sau.
// sources/types: nil = chạy tất cả; [] (rỗng) = bỏ qua phần đó; [a,b] = chỉ chạy a,b.
// Hợp nhất flow: thông tin đến trước tạo trước, thông tin sau cập nhật thêm.
func (s *CrmCustomerService) RebuildCrm(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, sources, types []string) (*crmdto.CrmRebuildResult, error) {
	result := &crmdto.CrmRebuildResult{}

	// Backfill: nil = tất cả, [] = bỏ qua
	if types == nil || len(types) > 0 {
		backfillResult, err := s.BackfillActivity(ctx, ownerOrgID, limit, types)
		if err != nil {
			return nil, err
		}
		result.Backfill = *backfillResult
	}

	// Sync: nil = tất cả, [] = bỏ qua
	if sources == nil || len(sources) > 0 {
		posCount, fbCount, err := s.SyncAllCustomers(ctx, ownerOrgID, sources)
		if err != nil {
			return nil, err
		}
		result.Sync = crmdto.CrmSyncResult{PosProcessed: posCount, FbProcessed: fbCount}
	}

	return result, nil
}
