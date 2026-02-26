// Package crmvc - Sync initial crm_customers từ pc_pos_customers và fb_customers.
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

// SyncAllCustomers đồng bộ khách từ POS và FB vào crm_customers.
// sources: []string{"pos","fb"} — rỗng hoặc nil = chạy tất cả (pos, fb).
// BẮT BUỘC: xử lý từ cũ đến mới (posData/panCakeData inserted_at, updated_at asc).
// Gọi khi cần sync lần đầu hoặc rebuild.
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
				if e := s.MergeFromPosCustomer(ctx, &doc); e == nil {
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

// RebuildCrm chạy sync rồi backfill. sources/types rỗng = tất cả.
// Thứ tự: sync profile trước, backfill activity sau.
func (s *CrmCustomerService) RebuildCrm(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, sources, types []string) (*crmdto.CrmRebuildResult, error) {
	posCount, fbCount, err := s.SyncAllCustomers(ctx, ownerOrgID, sources)
	if err != nil {
		return nil, err
	}
	backfillResult, err := s.BackfillActivity(ctx, ownerOrgID, limit, types)
	if err != nil {
		return nil, err
	}
	return &crmdto.CrmRebuildResult{
		Sync: crmdto.CrmSyncResult{
			PosProcessed: posCount,
			FbProcessed:  fbCount,
		},
		Backfill: *backfillResult,
	}, nil
}
