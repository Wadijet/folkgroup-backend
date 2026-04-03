// Package orderdatachanged — Đồng bộ đơn canonical commerce_orders từ mirror Pancake (pc_pos_orders), không phát EmitDataChanged (tránh lặp queue AID).
package orderdatachanged

import (
	"context"
	"strconv"
	"time"

	"meta_commerce/internal/api/events"
	ordermodels "meta_commerce/internal/api/order/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility/identity"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SyncCommerceOrderFromPancakeDataChange upsert commerce_orders từ event datachanged pc_pos_orders.
func SyncCommerceOrderFromPancakeDataChange(ctx context.Context, e events.DataChangeEvent) error {
	if e.Document == nil {
		return nil
	}
	b, err := bson.Marshal(e.Document)
	if err != nil {
		return err
	}
	var pos pcmodels.PcPosOrder
	if err := bson.Unmarshal(b, &pos); err != nil {
		return err
	}
	return UpsertCommerceFromPancakePosOrder(ctx, &pos)
}

// UpsertCommerceFromPancakePosOrder ghi commerce_orders theo bản ghi Pancake (gọi từ sync datachanged hoặc backfill sau).
func UpsertCommerceFromPancakePosOrder(ctx context.Context, pos *pcmodels.PcPosOrder) error {
	if pos == nil || pos.OwnerOrganizationID.IsZero() || pos.ID.IsZero() {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CommerceOrders)
	if !ok || coll == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	co := mapPancakeToCommerce(pos, now)
	filter := bson.M{
		"ownerOrganizationId":   pos.OwnerOrganizationID,
		"source":                ordermodels.SourcePancakePOS,
		"sourceRecordMongoId":   pos.ID,
	}
	set := bson.M{
		"uid":                 co.Uid,
		"source":              co.Source,
		"sourceIds":           co.SourceIds,
		"links":               co.Links,
		"ownerOrganizationId": co.OwnerOrganizationID,
		"sourceRecordMongoId": co.SourceRecordMongoID,
		"orderId":             co.OrderId,
		"status":              co.Status,
		"insertedAt":          co.InsertedAt,
		"posUpdatedAt":        co.PosUpdatedAt,
		"pageId":              co.PageId,
		"postId":              co.PostId,
		"customerId":          co.CustomerId,
		"posData":             co.PosData,
		"updatedAt":           now,
	}
	update := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(bson.M{
			"$set":         set,
			"$setOnInsert": bson.M{"createdAt": now},
		}).
		SetUpsert(true)
	_, err := coll.BulkWrite(ctx, []mongo.WriteModel{update}, options.BulkWrite().SetOrdered(true))
	return err
}

func mapPancakeToCommerce(pos *pcmodels.PcPosOrder, nowMs int64) *ordermodels.CommerceOrder {
	co := &ordermodels.CommerceOrder{
		Uid:                 pos.Uid,
		Source:              ordermodels.SourcePancakePOS,
		SourceIds:           cloneStringMap(pos.SourceIds),
		Links:               cloneLinkMap(pos.Links),
		OwnerOrganizationID: pos.OwnerOrganizationID,
		SourceRecordMongoID: pos.ID,
		OrderId:             pos.OrderId,
		Status:              pos.Status,
		InsertedAt:          pos.InsertedAt,
		PosUpdatedAt:        pos.PosUpdatedAt,
		PageId:              pos.PageId,
		PostId:              pos.PostId,
		CustomerId:          pos.CustomerId,
		PosData:             cloneIfaceMap(pos.PosData),
		CreatedAt:           nowMs,
		UpdatedAt:           nowMs,
	}
	if co.SourceIds == nil {
		co.SourceIds = map[string]string{}
	}
	if pos.OrderId != 0 {
		co.SourceIds["pos"] = strconv.FormatInt(pos.OrderId, 10)
	}
	return co
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneLinkMap(m map[string]identity.LinkItem) map[string]identity.LinkItem {
	if m == nil {
		return nil
	}
	raw, err := bson.Marshal(m)
	if err != nil {
		return nil
	}
	var out map[string]identity.LinkItem
	if err := bson.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func cloneIfaceMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	raw, err := bson.Marshal(m)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := bson.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
