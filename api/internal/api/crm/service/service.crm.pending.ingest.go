// Package crmvc - Service cho crm_pending_ingest: queue CRM cho worker xử lý.
package crmvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	basesvc "meta_commerce/internal/api/base/service"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// CrmPendingIngestService service CRUD cho crm_pending_ingest (dùng cho API đọc queue).
type CrmPendingIngestService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmPendingIngest]
}

// NewCrmPendingIngestService tạo service CRUD cho crm_pending_ingest.
func NewCrmPendingIngestService() (*CrmPendingIngestService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmPendingIngest)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CrmPendingIngest, common.ErrNotFound)
	}
	return &CrmPendingIngestService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmPendingIngest](coll),
	}, nil
}

// EnqueueCrmIngest thêm hoặc cập nhật job trong queue crm_pending_ingest (deduplicate theo businessKey).
// Cùng businessKey → upsert, chỉ giữ job mới nhất.
// prevDoc: optional — dùng để tính UpdatedAtDeltaMs (chênh lệch updated_at mới - cũ, ms).
func EnqueueCrmIngest(ctx context.Context, collectionName, operation string, document interface{}, prevDoc interface{}, ownerOrgID primitive.ObjectID) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmPendingIngest)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmPendingIngest)
	}
	docMap, err := documentToBsonM(document)
	if err != nil {
		return err
	}
	businessKey, ok := extractBusinessKey(collectionName, docMap, ownerOrgID)
	if !ok {
		return fmt.Errorf("không thể trích businessKey từ document")
	}
	now := time.Now().Unix()

	// Trích updated_at mới/cũ để debug và tính delta
	updatedAtNew := events.ExtractUpdatedAtFromDoc(collectionName, document)
	updatedAtOld := int64(0)
	if prevDoc != nil {
		updatedAtOld = events.ExtractUpdatedAtFromDoc(collectionName, prevDoc)
	}
	updatedAtDeltaMs := int64(-1)
	if updatedAtNew > 0 && updatedAtOld > 0 {
		updatedAtDeltaMs = updatedAtNew - updatedAtOld
	}

	// businessKey đã chứa collectionName (format: collectionName|org|part) nên chỉ cần filter theo businessKey
	filter := bson.M{"businessKey": businessKey}
	update := bson.M{
		"$set": bson.M{
			"collectionName":       collectionName,
			"operation":            operation,
			"document":             docMap,
			"ownerOrganizationId":  ownerOrgID,
			"createdAt":            now,
			"processedAt":          nil,
			"processError":         "",
			"updatedAtNew":         updatedAtNew,
			"updatedAtOld":         updatedAtOld,
			"updatedAtDeltaMs":     updatedAtDeltaMs,
		},
	}
	opts := mongoopts.Update().SetUpsert(true)
	_, err = coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// extractBusinessKey trích businessKey từ document để deduplicate queue.
// Trả về (key, true) hoặc ("", false) nếu không trích được.
func extractBusinessKey(collectionName string, docMap bson.M, ownerOrgID primitive.ObjectID) (string, bool) {
	orgHex := ownerOrgID.Hex()
	if orgHex == "" || orgHex == "000000000000000000000000" {
		return "", false
	}
	var part string
	switch collectionName {
	case global.MongoDB_ColNames.PcPosCustomers, global.MongoDB_ColNames.FbCustomers:
		part, _ = getStringFromMap(docMap, "customerId")
	case global.MongoDB_ColNames.PcPosOrders:
		if v, ok := docMap["orderId"]; ok {
			part = fmt.Sprintf("%v", v)
		}
	case global.MongoDB_ColNames.FbConvesations:
		part, _ = getStringFromMap(docMap, "conversationId")
	case global.MongoDB_ColNames.CrmNotes:
		if v, ok := docMap["_id"]; ok {
			if oid, ok := v.(primitive.ObjectID); ok {
				part = oid.Hex()
			}
		}
	}
	if part == "" {
		return "", false
	}
	return collectionName + "|" + orgHex + "|" + part, true
}

// documentToBsonM chuyển document (struct/ptr) sang bson.M để lưu queue.
func documentToBsonM(doc interface{}) (bson.M, error) {
	if doc == nil {
		return bson.M{}, nil
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var m bson.M
	if err := bson.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// CountUnprocessedCrmIngest đếm số job chưa xử lý trong queue (để adaptive batch, monitoring).
func CountUnprocessedCrmIngest(ctx context.Context) (int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmPendingIngest)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmPendingIngest)
	}
	return coll.CountDocuments(ctx, bson.M{"processedAt": nil})
}

// GetUnprocessedCrmIngest lấy tối đa limit job chưa xử lý, sort theo createdAt desc (job mới trước).
func GetUnprocessedCrmIngest(ctx context.Context, limit int) ([]crmmodels.CrmPendingIngest, error) {
	if limit <= 0 {
		limit = 50
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmPendingIngest)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmPendingIngest)
	}
	filter := bson.M{"processedAt": nil}
	opts := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var list []crmmodels.CrmPendingIngest
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []crmmodels.CrmPendingIngest{}
	}
	return list, nil
}

// SetCrmIngestProcessed đánh dấu job đã xử lý (thành công hoặc lỗi).
func SetCrmIngestProcessed(ctx context.Context, id primitive.ObjectID, processErr string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmPendingIngest)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmPendingIngest)
	}
	now := time.Now().Unix()
	update := bson.M{"$set": bson.M{"processedAt": now, "processError": processErr}}
	_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
